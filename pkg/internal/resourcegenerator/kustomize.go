package generator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/cookiegenerator"
	"k8s.io/api/apps/v1beta1"
	v1 "k8s.io/api/core/v1"
	rbacv1beta1 "k8s.io/api/rbac/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/kustomize/k8sdeps"
	"sigs.k8s.io/kustomize/pkg/commands/build"
	"sigs.k8s.io/kustomize/pkg/fs"
)

const stringLen = 24

//go:generate counterfeiter . ResourceGenerator

type ResourceGenerator interface {
	Build(GenerationContext) ([]TargetResource, error)
}

type KustomizeResourceGenerator struct {
	Filepath string
}

func NewKustomizeResourceGenerator(filepath string) *KustomizeResourceGenerator {
	return &KustomizeResourceGenerator{
		Filepath: filepath,
	}
}

func (k *KustomizeResourceGenerator) Build(generationContext GenerationContext) ([]TargetResource, error) {
	yamlString, err := k.parseYaml(generationContext)
	if err != nil {
		return nil, err
	}
	return decode(yamlString)
}

type GenerationContext struct {
	InstanceName string
	Namespace    string
	Nodes        int
}

type PatchJSON6902 struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

func (k *KustomizeResourceGenerator) parseYaml(generationContext GenerationContext) (string, error) {
	instanceName := generationContext.InstanceName
	namespace := generationContext.Namespace

	f := k8sdeps.NewFactory()
	filesystem := fs.MakeFakeFS()
	files, err := ioutil.ReadDir(k.Filepath)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		bytes, err := parseBytes(k.Filepath, file, instanceName, namespace)
		if err != nil {
			return "", err
		}
		filesystem.Mkdir("/base/")
		filesystem.WriteFile("/base/"+file.Name(), bytes)
	}

	populateOverlayDirectory(filesystem, generationContext)

	var out bytes.Buffer
	cmd := build.NewCmdBuild(&out, filesystem, f.ResmapF, f.TransformerF)
	cmd.SetArgs([]string{"/overlay"})
	cmd.SetOutput(ioutil.Discard)
	if _, err := cmd.ExecuteC(); err != nil {
		return "", err
	}
	output := out.String()

	return output, nil
}
func populateOverlayDirectory(filesystem fs.FileSystem, generationContext GenerationContext) error {

	filesystem.Mkdir("/overlay/")
	var kustomization bytes.Buffer
	fmt.Fprintf(&kustomization, "namePrefix: %s-\n", generationContext.InstanceName)
	fmt.Fprintf(&kustomization, "namespace: %s\n", generationContext.Namespace)
	kustomization.WriteString("commonLabels:\n")
	fmt.Fprintf(&kustomization, "  instance: %s\n", generationContext.InstanceName)
	kustomization.WriteString("bases:\n")
	kustomization.WriteString("- ../base\n")
	kustomization.WriteString("patchesJSON6902:\n")
	kustomization.WriteString("- target:\n")
	kustomization.WriteString("    group: apps\n")
	kustomization.WriteString("    version: v1beta1\n")
	kustomization.WriteString("    kind: StatefulSet\n")
	kustomization.WriteString("    name: rabbitmq\n")
	statefulSetPatch := []PatchJSON6902{
		{
			Op:    "replace",
			Path:  "/spec/replicas",
			Value: generationContext.Nodes,
		},
	}
	statefulSetPatchJson, parseErr := json.Marshal(statefulSetPatch)
	if parseErr != nil {
		return parseErr
	}

	statefulSetPatchName := "statefulset.json"
	if err := filesystem.WriteFile("/overlay/"+statefulSetPatchName, statefulSetPatchJson); err != nil {
		return err
	}

	fmt.Fprintf(&kustomization, "  path: %s\n", statefulSetPatchName)
	if err := filesystem.WriteFile("/overlay/kustomization.yaml", kustomization.Bytes()); err != nil {
		return err
	}
	return nil
}

func parseBytes(filepath string, file os.FileInfo, instanceName, namespace string) ([]byte, error) {
	bytes, err := ioutil.ReadFile(filepath + "/" + file.Name())
	if err != nil {
		return bytes, err
	}
	if file.Name() == "kustomization.yaml" {
		erlangCookie, err := cookiegenerator.Generate()
		if err != nil {
			return bytes, err
		}
		re := regexp.MustCompile("erlang-cookie=")
		bytes = re.ReplaceAll(bytes, []byte("erlang-cookie="+erlangCookie))
	}

	return bytes, nil
}

type TargetResource struct {
	ResourceObject runtime.Object
	EmptyResource  runtime.Object
	Name           string
	Namespace      string
}

func decode(yaml string) ([]TargetResource, error) {
	resources := strings.Split(yaml, "---")
	resourceArray := make([]TargetResource, 0, 9)
	for _, resource := range resources {

		decode := scheme.Codecs.UniversalDeserializer().Decode

		obj, _, err := decode([]byte(resource), nil, nil)
		if err != nil {
			fmt.Printf("%#v", err)
		}
		switch o := obj.(type) {
		case *v1.Secret:
			resourceArray = append(resourceArray, TargetResource{ResourceObject: obj, EmptyResource: &v1.Secret{}, Name: o.Name, Namespace: o.Namespace})
		case *v1.Service:
			resourceArray = append(resourceArray, TargetResource{ResourceObject: obj, EmptyResource: &v1.Service{}, Name: o.Name, Namespace: o.Namespace})
		case *v1.ConfigMap:
			resourceArray = append(resourceArray, TargetResource{ResourceObject: obj, EmptyResource: &v1.ConfigMap{}, Name: o.Name, Namespace: o.Namespace})
		case *v1beta1.StatefulSet:
			resourceArray = append(resourceArray, TargetResource{ResourceObject: obj, EmptyResource: &v1beta1.StatefulSet{}, Name: o.Name, Namespace: o.Namespace})
		case *rbacv1beta1.Role:
			resourceArray = append(resourceArray, TargetResource{ResourceObject: obj, EmptyResource: &rbacv1beta1.Role{}, Name: o.Name, Namespace: o.Namespace})
		case *rbacv1beta1.RoleBinding:
			resourceArray = append(resourceArray, TargetResource{ResourceObject: obj, EmptyResource: &rbacv1beta1.RoleBinding{}, Name: o.Name, Namespace: o.Namespace})
		case *v1.ServiceAccount:
			resourceArray = append(resourceArray, TargetResource{ResourceObject: obj, EmptyResource: &v1.ServiceAccount{}, Name: o.Name, Namespace: o.Namespace})

		default:
			return resourceArray, errors.New(fmt.Sprintf("Object unkown type: %s\n", reflect.TypeOf(obj)))

		}
	}
	return resourceArray, nil
}
