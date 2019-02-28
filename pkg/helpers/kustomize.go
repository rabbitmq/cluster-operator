package helpers

import (
	"bytes"
	"encoding/base64"
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
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/kustomize/k8sdeps"
	"sigs.k8s.io/kustomize/pkg/commands/build"
	"sigs.k8s.io/kustomize/pkg/fs"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func Build(filepath string, instanceName string, namespace string) error {
	f := k8sdeps.NewFactory()
	filesystem := fs.MakeFakeFS()
	files, err := ioutil.ReadDir(filepath)
	if err != nil {
		return err
	}

	for _, file := range files {
		bytes, err := ioutil.ReadFile(filepath + "/" + file.Name())
		if err != nil {
			return err
		}

		if file.Name() == "kustomization.yaml" {
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
		return err
	}
	output := out.String()

	resources := strings.Split(output, "---")
	for _, resource := range resources {
		Decode(resource)
	}
	return nil
}

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func Decode(yaml string) {
	decode := scheme.Codecs.UniversalDeserializer().Decode

	obj, _, err := decode([]byte(yaml), nil, nil)
	if err != nil {
		fmt.Printf("%#v", err)
	}

	switch o := obj.(type) {
	case *v1.Pod:
		fmt.Printf("%#v\n", o.Name)
	case *v1.Secret:
		fmt.Printf("%#v\n", o.Name)
	case *v1.Service:
		fmt.Printf("%#v\n", o.Name)
	case *v1.ConfigMap:
		fmt.Printf("%#v\n", o.Name)
	case *v1beta1.StatefulSet:
		fmt.Printf("%#v\n", o.Name)
	case *rbacv1beta1.Role:
		fmt.Printf("%#v\n", o.Name)
	case *rbacv1beta1.RoleBinding:
		fmt.Printf("%#v\n", o.Name)
	case *rbacv1.ClusterRole:
		fmt.Printf("%#v\n", o.Name)
	case *rbacv1.ClusterRoleBinding:
		fmt.Printf("%#v\n", o.Name)
	case *v1.ServiceAccount:
		fmt.Printf("%#v\n", o.Name)
	case *v1beta1.Deployment:
		fmt.Printf("%#v\n", o.Name)

	default:
		fmt.Printf("Object unkown type: %s\n", reflect.TypeOf(obj))
	}
}
