package internal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	. "github.com/rabbitmq/cluster-operator/internal/topology"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GenerateTopicPermissions", func() {
	var p *rabbitmqv1beta1.TopicPermission

	BeforeEach(func() {
		p = &rabbitmqv1beta1.TopicPermission{
			ObjectMeta: metav1.ObjectMeta{
				Name: "perm",
			},
			Spec: rabbitmqv1beta1.TopicPermissionSpec{
				User:  "a-user",
				Vhost: "/new-vhost",
			},
		}
	})

	It("sets 'Exchange' correctly", func() {
		p.Spec.Permissions.Exchange = "a-random-exchange"
		rmqPermissions := GenerateTopicPermissions(p)
		Expect(rmqPermissions.Exchange).To(Equal("a-random-exchange"))
	})

	It("sets 'Write' correctly", func() {
		p.Spec.Permissions.Write = ".~"
		rmqPermissions := GenerateTopicPermissions(p)
		Expect(rmqPermissions.Write).To(Equal(".~"))
	})

	It("sets 'Read' correctly", func() {
		p.Spec.Permissions.Read = "^$"
		rmqPermissions := GenerateTopicPermissions(p)
		Expect(rmqPermissions.Read).To(Equal("^$"))
	})
})
