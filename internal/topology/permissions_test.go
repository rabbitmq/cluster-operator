package internal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	. "github.com/rabbitmq/cluster-operator/internal/topology"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GeneratePermissions", func() {
	var p *rabbitmqv1beta1.Permission

	BeforeEach(func() {
		p = &rabbitmqv1beta1.Permission{
			ObjectMeta: metav1.ObjectMeta{
				Name: "user-permissions",
			},
			Spec: rabbitmqv1beta1.PermissionSpec{
				User:  "a-user",
				Vhost: "/new-vhost",
			},
		}
	})

	It("sets 'Configure' correctly", func() {
		p.Spec.Permissions.Configure = ".*"
		rmqPermissions := GeneratePermissions(p)
		Expect(rmqPermissions.Configure).To(Equal(".*"))
	})

	It("sets 'Write' correctly", func() {
		p.Spec.Permissions.Write = ".~"
		rmqPermissions := GeneratePermissions(p)
		Expect(rmqPermissions.Write).To(Equal(".~"))
	})

	It("sets 'Read' correctly", func() {
		p.Spec.Permissions.Read = "^$"
		rmqPermissions := GeneratePermissions(p)
		Expect(rmqPermissions.Read).To(Equal("^$"))
	})
})
