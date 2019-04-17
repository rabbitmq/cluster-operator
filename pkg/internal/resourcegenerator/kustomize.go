package generator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"

	yaml "gopkg.in/yaml.v2"
	"sigs.k8s.io/kustomize/pkg/gvk"
	"sigs.k8s.io/kustomize/pkg/patch"

	"k8s.io/api/apps/v1beta1"
	v1 "k8s.io/api/core/v1"
	rbacv1beta1 "k8s.io/api/rbac/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/kustomize/k8sdeps"
	"sigs.k8s.io/kustomize/pkg/commands/build"
	"sigs.k8s.io/kustomize/pkg/fs"
	"sigs.k8s.io/kustomize/pkg/types"
)

type GenerationContext struct {
	InstanceName string
	Namespace    string
	Nodes        int32
}

type PatchJSON6902 struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

//go:generate counterfeiter . ResourceGenerator

type ResourceGenerator interface {
	Build(generationContext GenerationContext) ([]TargetResource, error)
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
	yamlString, err := k.parseYamlString(generationContext)
	if err != nil {
		return nil, err
	}
	return decodeToObjects(yamlString)
}

func (k *KustomizeResourceGenerator) parseYamlString(generationContext GenerationContext) (string, error) {
	filesystem := fs.MakeFakeFS()

	if err := k.populateBaseDirectory(filesystem, generationContext); err != nil {
		return "", err
	}
	if err := populateOverlayDirectory(filesystem, generationContext); err != nil {
		return "", err
	}

	var out bytes.Buffer
	f := k8sdeps.NewFactory()
	cmd := build.NewCmdBuild(&out, filesystem, f.ResmapF, f.TransformerF)
	cmd.SetArgs([]string{"/overlay"})
	cmd.SetOutput(ioutil.Discard)
	if _, err := cmd.ExecuteC(); err != nil {
		return "", err
	}
	output := out.String()

	return output, nil
}

func (k *KustomizeResourceGenerator) populateBaseDirectory(filesystem fs.FileSystem, generationContext GenerationContext) error {
	files, err := ioutil.ReadDir(k.Filepath)
	if err != nil {
		return err
	}

	for _, file := range files {
		bytes, err := parseBytes(k.Filepath, file, generationContext.InstanceName, generationContext.Namespace)
		if err != nil {
			return err
		}
		filesystem.Mkdir("/base/")
		filesystem.WriteFile("/base/"+file.Name(), bytes)
	}
	return nil
}
func populateOverlayDirectory(filesystem fs.FileSystem, generationContext GenerationContext) error {
	filesystem.Mkdir("/overlay/")
	statefulSetPatchName := "statefulset.json"
	writePatchFile(statefulSetPatchName, generationContext, filesystem)
	writeKustomizationFile(statefulSetPatchName, generationContext, filesystem)
	return nil
}
func writePatchFile(filename string, generationContext GenerationContext, filesystem fs.FileSystem) error {
	//TODO Check if there's a way to do a replace on an array that is not by index
	patch := []PatchJSON6902{
		{
			Op:    "replace",
			Path:  "/spec/replicas",
			Value: generationContext.Nodes,
		},
		{
			Op:    "replace",
			Path:  "/spec/template/spec/containers/0/env/0/valueFrom/secretKeyRef/name",
			Value: generationContext.InstanceName,
		},
	}
	patchJson, parseErr := json.Marshal(patch)
	if parseErr != nil {
		return parseErr
	}

	if err := filesystem.WriteFile("/overlay/"+filename, patchJson); err != nil {
		return err
	}
	return nil
}

func writeKustomizationFile(patchFileName string, generationContext GenerationContext, filesystem fs.FileSystem) error {
	kustomize := &types.Kustomization{
		NamePrefix: "p-" + generationContext.InstanceName + "-",
		Namespace:  generationContext.Namespace,
		CommonLabels: map[string]string{
			"instance": generationContext.InstanceName,
		},
		Bases: []string{"../base"},
		PatchesJson6902: []patch.Json6902{
			{
				Path: patchFileName,
				Target: &patch.Target{
					Gvk: gvk.Gvk{
						Group:   "apps",
						Version: "v1beta1",
						Kind:    "StatefulSet",
					},
					Name: "rabbitmq",
				},
			},
		},
	}
	kustomizeYaml, err := yaml.Marshal(kustomize)
	if err != nil {
		return err
	}
	if err := filesystem.WriteFile("/overlay/kustomization.yaml", kustomizeYaml); err != nil {
		return err
	}
	return nil
}

func parseBytes(filepath string, file os.FileInfo, instanceName, namespace string) ([]byte, error) {
	bytes, err := ioutil.ReadFile(filepath + "/" + file.Name())
	if err != nil {
		return bytes, err
	}

	return bytes, nil
}

type TargetResource struct {
	ResourceObject runtime.Object
	EmptyResource  runtime.Object
	Name           string
	Namespace      string
}

func decodeToObjects(yaml string) ([]TargetResource, error) {
	resources := strings.Split(yaml, "---")
	resourceArray := make([]TargetResource, 0, 9)
	for _, resource := range resources {

		decode := scheme.Codecs.UniversalDeserializer().Decode

		obj, _, err := decode([]byte(resource), nil, nil)
		if err != nil {
			fmt.Printf("%#v", err)
		}
		switch o := obj.(type) {
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
