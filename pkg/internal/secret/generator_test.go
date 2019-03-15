package secret_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/pkg/apis/rabbitmq/v1beta1"
	. "github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/secret"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("SecretGenerator", func() {
	var (
		instance *rabbitmqv1beta1.RabbitmqCluster
		secret   Secret
	)

	Describe("Generate", func() {
		BeforeEach(func() {
			instance = &rabbitmqv1beta1.RabbitmqCluster{
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Plan: "plan1",
				},
				Status: rabbitmqv1beta1.RabbitmqClusterStatus{},
				TypeMeta: metav1.TypeMeta{
					Kind:       "RabbitmqCluster",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq",
					Namespace: "rabbitmq",
				},
			}
			secret = &RabbitSecret{}
		})
		It("generates different passwords each time", func() {

			first, err := secret.New(instance)
			Expect(err).NotTo(HaveOccurred())

			second, err2 := secret.New(instance)
			Expect(err2).NotTo(HaveOccurred())

			Expect(first.Data["erlang-cookie"]).NotTo(Equal(second.Data["erlang-cookie"]))
		})

		It("generates url safe passwords", func() {
			secret, err := secret.New(instance)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(secret.Data["erlang-cookie"])).To(MatchRegexp("^[a-zA-Z0-9\\-_]{24}$"))
		})
	})
})
