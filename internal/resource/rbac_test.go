package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
)

var _ = Describe("RBAC", func() {
	var instance = rabbitmqv1beta1.RabbitmqCluster{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "a-namespace",
			Name:      "a-name",
		},
	}

	Describe("GenerateServiceAccount", func() {
		It("generates a correct service account", func() {
			serviceAccount := resource.GenerateServiceAccount(instance)
			Expect(serviceAccount.Namespace).To(Equal(instance.Namespace))
			Expect(serviceAccount.Name).To(Equal(instance.ChildResourceName("rabbitmq-server")))
		})
	})

	Describe("GenerateRole", func() {
		It("generates a correct service account", func() {
			role := resource.GenerateRole(instance)
			Expect(role.Namespace).To(Equal(instance.Namespace))
			Expect(role.Name).To(Equal(instance.ChildResourceName("rabbitmq-endpoint-discovery")))

			Expect(len(role.Rules)).To(Equal(1))

			rule := role.Rules[0]
			Expect(rule.APIGroups).To(Equal([]string{""}))
			Expect(rule.Resources).To(Equal([]string{"endpoints"}))
			Expect(rule.Verbs).To(Equal([]string{"get"}))
		})
	})

	Describe("GenerateRoleBinding", func() {
		It("generates a correct service account", func() {
			roleBinding := resource.GenerateRoleBinding(instance)
			Expect(roleBinding.Namespace).To(Equal(instance.Namespace))
			Expect(roleBinding.Name).To(Equal(instance.ChildResourceName("rabbitmq-server")))

			Expect(len(roleBinding.Subjects)).To(Equal(1))
			subject := roleBinding.Subjects[0]

			Expect(subject.Kind).To(Equal("ServiceAccount"))
			Expect(subject.Name).To(Equal(instance.ChildResourceName("rabbitmq-server")))

			Expect(roleBinding.RoleRef.APIGroup).To(Equal("rbac.authorization.k8s.io"))
			Expect(roleBinding.RoleRef.Kind).To(Equal("Role"))
			Expect(roleBinding.RoleRef.Name).To(Equal(instance.ChildResourceName("rabbitmq-endpoint-discovery")))
		})
	})
})
