package generator

import (
	"bytes"
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
	Build(string, string) ([]TargetResource, error)
}

type KustomizeResourceGenerator struct {
	Filepath string
}

func NewKustomizeResourceGenerator(filepath string) *KustomizeResourceGenerator {
	return &KustomizeResourceGenerator{
		Filepath: filepath,
	}
}

func (k *KustomizeResourceGenerator) Build(instanceName string, namespace string) ([]TargetResource, error) {
	yamlString, err := k.parseYaml(instanceName, namespace)
	if err != nil {
		return nil, err
	}
	return decode(yamlString)
}

func (k *KustomizeResourceGenerator) parseYaml(instanceName string, namespace string) (string, error) {
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
	var kustomization bytes.Buffer
	fmt.Fprintf(&kustomization, "namePrefix: %s-\n", instanceName)
	fmt.Fprintf(&kustomization, "namespace: %s\n", namespace)
	kustomization.WriteString("commonLabels:\n")
	fmt.Fprintf(&kustomization, "  instance: %s\n", instanceName)
	kustomization.WriteString("bases:\n")
	kustomization.WriteString("- ../base\n")

	filesystem.Mkdir("/overlay/")
	if err := filesystem.WriteFile("/overlay/kustomization.yaml", kustomization.Bytes()); err != nil {
		return "", err
	}

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
