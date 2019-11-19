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

var _ = Describe("RabbitmqCluster", func() {
	Context("Resources", func() {
		var (
			instance = rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test",
					Namespace: "namespace",
				},
			}

			rabbitmqCluster *resource.RabbitmqCluster
			scheme          *runtime.Scheme
		)
		When("an operator Registry secret is set in the default configuration", func() {
			BeforeEach(func() {
				scheme = runtime.NewScheme()
				rabbitmqv1beta1.AddToScheme(scheme)
				defaultscheme.AddToScheme(scheme)

				rabbitmqCluster = &resource.RabbitmqCluster{
					Instance:             &instance,
					DefaultConfiguration: resource.DefaultConfiguration{OperatorRegistrySecret: &corev1.Secret{}},
					StatefulSetConfiguration: resource.StatefulSetConfiguration{
						PersistenceStorageClassName: "standard",
						PersistenceStorage:          "10Gi",
						Scheme:                      scheme,
					},
				}
			})
			It("returns the required resources", func() {
				resources, err := rabbitmqCluster.Resources()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(resources)).To(Equal(10))

				resourceMap := checkForResources(resources)

				expectedKeys := []string{
					"ConfigMap:test-rabbitmq-server-conf",
					"Role:test-rabbitmq-endpoint-discovery",
					"RoleBinding:test-rabbitmq-server",
					"Secret:test-rabbitmq-admin",
					"Secret:test-rabbitmq-erlang-cookie",
					"Secret:test-registry-access",
					"Service:test-rabbitmq-headless",
					"Service:test-rabbitmq-ingress",
					"ServiceAccount:test-rabbitmq-server",
					"StatefulSet:test-rabbitmq-server",
				}

				for index, _ := range expectedKeys {
					Expect(resourceMap[expectedKeys[index]]).Should(BeTrue())
				}

				Expect(len(resourceMap)).To(Equal(10))
			})
		})

		When("no operator registry secret is set in the default configuration", func() {
			BeforeEach(func() {
				scheme = runtime.NewScheme()
				rabbitmqv1beta1.AddToScheme(scheme)
				defaultscheme.AddToScheme(scheme)

				rabbitmqCluster = &resource.RabbitmqCluster{
					Instance:             &instance,
					DefaultConfiguration: resource.DefaultConfiguration{},
					StatefulSetConfiguration: resource.StatefulSetConfiguration{
						PersistenceStorageClassName: "standard",
						PersistenceStorage:          "10Gi",
						Scheme:                      scheme,
					},
				}
			})
			It("returns the required resources", func() {
				resources, err := rabbitmqCluster.Resources()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(resources)).To(Equal(9))

				resourceMap := checkForResources(resources)

				expectedKeys := []string{
					"ConfigMap:test-rabbitmq-server-conf",
					"Role:test-rabbitmq-endpoint-discovery",
					"RoleBinding:test-rabbitmq-server",
					"Secret:test-rabbitmq-admin",
					"Secret:test-rabbitmq-erlang-cookie",
					"Service:test-rabbitmq-headless",
					"Service:test-rabbitmq-ingress",
					"ServiceAccount:test-rabbitmq-server",
					"StatefulSet:test-rabbitmq-server",
				}

				for index, _ := range expectedKeys {
					Expect(resourceMap[expectedKeys[index]]).Should(BeTrue())
				}

				Expect(len(resourceMap)).To(Equal(9))

			})
		})
	})
})

func checkForResources(resources []runtime.Object) (resourceMap map[string]bool) {
	resourceMap = make(map[string]bool)
	for _, resource := range resources {
		switch r := resource.(type) {
		case *corev1.Secret:
			if r.Name == "test-rabbitmq-admin" {
				resourceMap[fmt.Sprintf("Secret:%s", r.Name)] = true
			}
			if r.Name == "test-rabbitmq-erlang-cookie" {
				resourceMap[fmt.Sprintf("Secret:%s", r.Name)] = true
			}
			if r.Name == "test-registry-access" {
				resourceMap[fmt.Sprintf("Secret:%s", r.Name)] = true
			}
		case *corev1.Service:
			if r.Name == "test-rabbitmq-headless" {
				resourceMap[fmt.Sprintf("Service:%s", r.Name)] = true
			}
			if r.Name == "test-rabbitmq-ingress" {
				resourceMap[fmt.Sprintf("Service:%s", r.Name)] = true
			}
		case *corev1.ConfigMap:
			if r.Name == "test-rabbitmq-server-conf" {
				resourceMap[fmt.Sprintf("ConfigMap:%s", r.Name)] = true
			}
		case *corev1.ServiceAccount:
			if r.Name == "test-rabbitmq-server" {
				resourceMap[fmt.Sprintf("ServiceAccount:%s", r.Name)] = true
			}
		case *rbacv1.Role:
			if r.Name == "test-rabbitmq-endpoint-discovery" {
				resourceMap[fmt.Sprintf("Role:%s", r.Name)] = true
			}
		case *rbacv1.RoleBinding:
			if r.Name == "test-rabbitmq-server" {
				resourceMap[fmt.Sprintf("RoleBinding:%s", r.Name)] = true
			}
		case *appsv1.StatefulSet:
			if r.Name == "test-rabbitmq-server" {
				resourceMap[fmt.Sprintf("StatefulSet:%s", r.Name)] = true
			}
		}
	}
	return resourceMap
}
