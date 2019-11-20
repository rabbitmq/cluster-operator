package resource_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("RabbitmqResourceBuilder", func() {
	Context("Resources", func() {
		var (
			instance = rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test",
					Namespace: "namespace",
				},
			}

			rabbitmqCluster *resource.RabbitmqResourceBuilder
			scheme          *runtime.Scheme
		)
		When("an operator Registry secret is set in the default configuration", func() {
			BeforeEach(func() {
				scheme = runtime.NewScheme()
				rabbitmqv1beta1.AddToScheme(scheme)
				defaultscheme.AddToScheme(scheme)

				rabbitmqCluster = &resource.RabbitmqResourceBuilder{
					Instance: &instance,
					DefaultConfiguration: resource.DefaultConfiguration{
						OperatorRegistrySecret: &corev1.Secret{}, PersistentStorageClassName: "standard",
						PersistentStorage: "10Gi",
						Scheme:            scheme},
				}
			})
			It("returns the required resources in the expected order", func() {
				resources, err := rabbitmqCluster.Resources()
				Expect(err).NotTo(HaveOccurred())

				Expect(len(resources)).To(Equal(10))

				resourceMap := checkForResources(resources)

				expectedKeys := []string{
					"0 - ConfigMap:test-rabbitmq-server-conf",
					"1 - Service:test-rabbitmq-ingress",
					"2 - Service:test-rabbitmq-headless",
					"3 - Secret:test-rabbitmq-admin",
					"4 - Secret:test-rabbitmq-erlang-cookie",
					"5 - Secret:test-registry-access",
					"6 - ServiceAccount:test-rabbitmq-server",
					"7 - Role:test-rabbitmq-endpoint-discovery",
					"8 - RoleBinding:test-rabbitmq-server",
					"9 - StatefulSet:test-rabbitmq-server",
				}

				for index, _ := range expectedKeys {
					Expect(resourceMap[expectedKeys[index]]).Should(BeTrue())
				}
			})
		})

		When("no operator registry secret is set in the default configuration", func() {
			BeforeEach(func() {
				scheme = runtime.NewScheme()
				rabbitmqv1beta1.AddToScheme(scheme)
				defaultscheme.AddToScheme(scheme)

				rabbitmqCluster = &resource.RabbitmqResourceBuilder{
					Instance: &instance,
					DefaultConfiguration: resource.DefaultConfiguration{
						PersistentStorageClassName: "standard",
						PersistentStorage:          "10Gi",
						Scheme:                     scheme,
					},
				}
			})
			It("returns the required resources in the expected order", func() {
				resources, err := rabbitmqCluster.Resources()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(resources)).To(Equal(9))

				resourceMap := checkForResources(resources)

				expectedKeys := []string{
					"0 - ConfigMap:test-rabbitmq-server-conf",
					"1 - Service:test-rabbitmq-ingress",
					"2 - Service:test-rabbitmq-headless",
					"3 - Secret:test-rabbitmq-admin",
					"4 - Secret:test-rabbitmq-erlang-cookie",
					"5 - ServiceAccount:test-rabbitmq-server",
					"6 - Role:test-rabbitmq-endpoint-discovery",
					"7 - RoleBinding:test-rabbitmq-server",
					"8 - StatefulSet:test-rabbitmq-server",
				}

				for index, _ := range expectedKeys {
					Expect(resourceMap[expectedKeys[index]]).Should(BeTrue())
				}
			})
		})
	})
})

func checkForResources(resources []runtime.Object) (resourceMap map[string]bool) {
	resourceMap = make(map[string]bool)
	for i, resource := range resources {
		switch r := resource.(type) {
		case *corev1.Secret:
			if r.Name == "test-rabbitmq-admin" {
				resourceMap[fmt.Sprintf("%d - Secret:%s", i, r.Name)] = true
			}
			if r.Name == "test-rabbitmq-erlang-cookie" {
				resourceMap[fmt.Sprintf("%d - Secret:%s", i, r.Name)] = true
			}
			if r.Name == "test-registry-access" {
				resourceMap[fmt.Sprintf("%d - Secret:%s", i, r.Name)] = true
			}
		case *corev1.Service:
			if r.Name == "test-rabbitmq-headless" {
				resourceMap[fmt.Sprintf("%d - Service:%s", i, r.Name)] = true
			}
			if r.Name == "test-rabbitmq-ingress" {
				resourceMap[fmt.Sprintf("%d - Service:%s", i, r.Name)] = true
			}
		case *corev1.ConfigMap:
			if r.Name == "test-rabbitmq-server-conf" {
				resourceMap[fmt.Sprintf("%d - ConfigMap:%s", i, r.Name)] = true
			}
		case *corev1.ServiceAccount:
			if r.Name == "test-rabbitmq-server" {
				resourceMap[fmt.Sprintf("%d - ServiceAccount:%s", i, r.Name)] = true
			}
		case *rbacv1.Role:
			if r.Name == "test-rabbitmq-endpoint-discovery" {
				resourceMap[fmt.Sprintf("%d - Role:%s", i, r.Name)] = true
			}
		case *rbacv1.RoleBinding:
			if r.Name == "test-rabbitmq-server" {
				resourceMap[fmt.Sprintf("%d - RoleBinding:%s", i, r.Name)] = true
			}
		case *appsv1.StatefulSet:
			if r.Name == "test-rabbitmq-server" {
				resourceMap[fmt.Sprintf("%d - StatefulSet:%s", i, r.Name)] = true
			}
		}
	}
	return resourceMap
}
