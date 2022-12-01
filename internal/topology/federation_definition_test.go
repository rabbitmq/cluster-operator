package internal

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GenerationFederationDefinition", func() {
	var f *rabbitmqv1beta1.Federation

	BeforeEach(func() {
		f = &rabbitmqv1beta1.Federation{
			ObjectMeta: metav1.ObjectMeta{
				Name: "new-federation",
			},
			Spec: rabbitmqv1beta1.FederationSpec{
				Vhost: "/new-vhost",
				Name:  "new-federation",
			},
		}
	})

	It("sets 'uri' correctly for a single uri", func() {
		definition := GenerateFederationDefinition(f, "a-rabbitmq-uri@test.com")
		Expect(definition.Uri).To(ConsistOf("a-rabbitmq-uri@test.com"))
	})

	It("sets 'uri' correctly for multiple uris", func() {
		definition := GenerateFederationDefinition(f, "a-rabbitmq-uri@test.com0,a-rabbitmq-uri@test1.com,a-rabbitmq-uri@test2.com")
		Expect(definition.Uri).To(ConsistOf("a-rabbitmq-uri@test.com0", "a-rabbitmq-uri@test1.com", "a-rabbitmq-uri@test2.com"))
	})

	It("sets 'PrefetchCount' correctly", func() {
		f.Spec.PrefetchCount = 200
		definition := GenerateFederationDefinition(f, "")
		Expect(definition.PrefetchCount).To(Equal(200))
	})

	It("sets 'AckMode' correctly", func() {
		f.Spec.AckMode = "no-ack"
		definition := GenerateFederationDefinition(f, "")
		Expect(definition.AckMode).To(Equal("no-ack"))
	})

	It("sets 'Expires' correctly", func() {
		f.Spec.Expires = 100000
		definition := GenerateFederationDefinition(f, "")
		Expect(definition.Expires).To(Equal(100000))
	})

	It("sets 'MessageTTL' correctly", func() {
		f.Spec.MessageTTL = 300
		definition := GenerateFederationDefinition(f, "")
		Expect(definition.MessageTTL).To(Equal(int32(300)))
	})

	It("sets 'MaxHops' correctly", func() {
		f.Spec.MaxHops = 5
		definition := GenerateFederationDefinition(f, "")
		Expect(definition.MaxHops).To(Equal(5))
	})

	It("sets 'ReconnectDelay' correctly", func() {
		f.Spec.ReconnectDelay = 100
		definition := GenerateFederationDefinition(f, "")
		Expect(definition.ReconnectDelay).To(Equal(100))
	})

	It("sets 'TrustUserId' correctly", func() {
		f.Spec.TrustUserId = false
		definition := GenerateFederationDefinition(f, "")
		Expect(definition.TrustUserId).To(BeFalse())
	})

	It("sets 'Exchange' correctly", func() {
		f.Spec.Exchange = "an-exchange"
		definition := GenerateFederationDefinition(f, "")
		Expect(definition.Exchange).To(Equal("an-exchange"))
	})

	It("sets 'Queue' correctly", func() {
		f.Spec.Queue = "a-great-queue"
		definition := GenerateFederationDefinition(f, "")
		Expect(definition.Queue).To(Equal("a-great-queue"))
	})
})
