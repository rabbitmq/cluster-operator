package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
)

var _ = Describe("RBAC", func() {
	var (
		instance rabbitmqv1beta1.RabbitmqCluster
		cluster  *resource.RabbitmqResourceBuilder
	)

	BeforeEach(func() {
		instance = rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: v1.ObjectMeta{
				Namespace: "a-namespace",
				Name:      "a-name",
			},
		}
		cluster = &resource.RabbitmqResourceBuilder{
			Instance: &instance,
		}
	})

	Describe("GenerateRoleBinding", func() {
		var roleBinding *rbacv1.RoleBinding
		BeforeEach(func() {
			cluster = &resource.RabbitmqResourceBuilder{
				Instance: &instance,
			}
			roleBinding = cluster.RoleBinding()
		})
		It("generates a correct service account", func() {
			Expect(roleBinding.Namespace).To(Equal(cluster.Instance.Namespace))
			Expect(roleBinding.Name).To(Equal(cluster.Instance.ChildResourceName("server")))

			Expect(len(roleBinding.Subjects)).To(Equal(1))
			subject := roleBinding.Subjects[0]

			Expect(subject.Kind).To(Equal("ServiceAccount"))
			Expect(subject.Name).To(Equal(cluster.Instance.ChildResourceName("server")))

			Expect(roleBinding.RoleRef.APIGroup).To(Equal("rbac.authorization.k8s.io"))
			Expect(roleBinding.RoleRef.Kind).To(Equal("Role"))
			Expect(roleBinding.RoleRef.Name).To(Equal(cluster.Instance.ChildResourceName("endpoint-discovery")))
		})
		It("adds the required labels", func() {
			labels := roleBinding.Labels
			Expect(labels["app.kubernetes.io/name"]).To(Equal(cluster.Instance.Name))
			Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
		})
		Context("label inheritance", func() {
			BeforeEach(func() {
				instance.Labels = map[string]string{
					"app.kubernetes.io/foo": "bar",
					"foo":                   "bar",
					"rabbitmq":              "is-great",
					"foo/app.kubernetes.io": "edgecase",
				}
			})

			It("has the labels from the CRD on the rolebinding", func() {
				roleBinding := cluster.RoleBinding()
				testLabels(roleBinding.Labels)
			})
		})
	})
})
