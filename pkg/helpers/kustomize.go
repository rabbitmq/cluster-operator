package helpers

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"reflect"
	"regexp"
	"strings"
	"time"

	"k8s.io/api/apps/v1beta1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	rbacv1beta1 "k8s.io/api/rbac/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/kustomize/k8sdeps"
	"sigs.k8s.io/kustomize/pkg/commands/build"
	"sigs.k8s.io/kustomize/pkg/fs"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func Build(filepath string, instanceName string, namespace string) (string, error) {
	f := k8sdeps.NewFactory()
	filesystem := fs.MakeFakeFS()
	files, err := ioutil.ReadDir(filepath)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		bytes, err := ioutil.ReadFile(filepath + "/" + file.Name())
		if err != nil {
			return "", err
		}

		if file.Name() == "kustomization.yaml" {
			// TODO: This needs to be more secure
			rand.Seed(time.Now().UnixNano())
			randomString := RandStringRunes(20)
			erlangCookie := base64.StdEncoding.EncodeToString([]byte(randomString))
			var replaces = []struct {
				regexp string
				value  string
			}{
				{"namePrefix: .*", "namePrefix: " + instanceName + "-"},
				{"namespace: .*", "namespace: " + namespace},
				{"instance: .*", "instance: " + instanceName},
				{"erlang-cookie=", "erlang-cookie=" + erlangCookie},
			}
			for _, replace := range replaces {
				re := regexp.MustCompile(replace.regexp)
				value := replace.value
				bytes = re.ReplaceAll(bytes, []byte(value))
			}
		}

		filesystem.WriteFile("/"+file.Name(), bytes)
	}

	var out bytes.Buffer
	cmd := build.NewCmdBuild(&out, filesystem, f.ResmapF, f.TransformerF)
	cmd.SetArgs([]string{"/"})
	cmd.SetOutput(ioutil.Discard)
	if _, err := cmd.ExecuteC(); err != nil {
		return "", err
	}
	output := out.String()

	return output, nil
}

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

type TargetResource struct {
	ResourceObject runtime.Object
	EmptyResource  runtime.Object
	Name           string
	Namespace      string
}

func Decode(yaml string) ([]TargetResource, error) {
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
		case *rbacv1.ClusterRole:
			resourceArray = append(resourceArray, TargetResource{ResourceObject: obj, EmptyResource: &rbacv1.ClusterRole{}, Name: o.Name, Namespace: o.Namespace})
		case *rbacv1.ClusterRoleBinding:
			resourceArray = append(resourceArray, TargetResource{ResourceObject: obj, EmptyResource: &rbacv1.ClusterRoleBinding{}, Name: o.Name, Namespace: o.Namespace})
		case *v1.ServiceAccount:
			resourceArray = append(resourceArray, TargetResource{ResourceObject: obj, EmptyResource: &v1.ServiceAccount{}, Name: o.Name, Namespace: o.Namespace})

		default:
			return resourceArray, errors.New(fmt.Sprintf("Object unkown type: %s\n", reflect.TypeOf(obj)))

		}
	}
	return resourceArray, nil
}
