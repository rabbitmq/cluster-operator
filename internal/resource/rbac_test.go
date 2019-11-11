package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
)

var _ = Describe("RBAC", func() {
	var (
		instance       rabbitmqv1beta1.RabbitmqCluster
		serviceAccount *corev1.ServiceAccount
	)
	BeforeEach(func() {
		instance = rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: v1.ObjectMeta{
				Namespace: "a-namespace",
				Name:      "a-name",
			},
		}
		serviceAccount = resource.GenerateServiceAccount(instance)
	})

	Describe("GenerateServiceAccount", func() {
		It("generates a correct service account", func() {
			Expect(serviceAccount.Namespace).To(Equal(instance.Namespace))
			Expect(serviceAccount.Name).To(Equal(instance.ChildResourceName("server")))
		})

		It("adds the required labels", func() {
			labels := serviceAccount.Labels
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
			Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
		})
	})

	Describe("GenerateRole", func() {
		var role *rbacv1.Role
		BeforeEach(func() {
			role = resource.GenerateRole(instance)
		})
		It("generates a correct service account", func() {
			Expect(role.Namespace).To(Equal(instance.Namespace))
			Expect(role.Name).To(Equal(instance.ChildResourceName("endpoint-discovery")))

			Expect(len(role.Rules)).To(Equal(1))

			rule := role.Rules[0]
			Expect(rule.APIGroups).To(Equal([]string{""}))
			Expect(rule.Resources).To(Equal([]string{"endpoints"}))
			Expect(rule.Verbs).To(Equal([]string{"get"}))
		})
		It("adds the required labels", func() {
			labels := role.Labels
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
			Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
		})
	})

	Describe("GenerateRoleBinding", func() {
		var roleBinding *rbacv1.RoleBinding
		BeforeEach(func() {
			roleBinding = resource.GenerateRoleBinding(instance)
		})
		It("generates a correct service account", func() {
			Expect(roleBinding.Namespace).To(Equal(instance.Namespace))
			Expect(roleBinding.Name).To(Equal(instance.ChildResourceName("server")))

			Expect(len(roleBinding.Subjects)).To(Equal(1))
			subject := roleBinding.Subjects[0]

			Expect(subject.Kind).To(Equal("ServiceAccount"))
			Expect(subject.Name).To(Equal(instance.ChildResourceName("server")))

			Expect(roleBinding.RoleRef.APIGroup).To(Equal("rbac.authorization.k8s.io"))
			Expect(roleBinding.RoleRef.Kind).To(Equal("Role"))
			Expect(roleBinding.RoleRef.Name).To(Equal(instance.ChildResourceName("endpoint-discovery")))
		})
		It("adds the required labels", func() {
			labels := roleBinding.Labels
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
			Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
		})
	})
})
