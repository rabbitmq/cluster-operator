package resource_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		)

		BeforeEach(func() {
			rabbitmqCluster = &resource.RabbitmqCluster{
				Instance: &instance,
			}
		})
		It("returns the required resources", func() {
			resources, err := rabbitmqCluster.Resources()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(resources)).To(Equal(8))

			resourceMap := make(map[string]bool)
			for _, resource := range resources {
				switch r := resource.(type) {
				case *corev1.Secret:
					if r.Name == "test-rabbitmq-admin" {
						resourceMap[fmt.Sprintf("Secret:%s", r.Name)] = true
					}
					if r.Name == "test-rabbitmq-erlang-cookie" {
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

				}
			}

			Expect(len(resourceMap)).To(Equal(8))

		})
	})
})
