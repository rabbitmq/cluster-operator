package internal

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GenerateShovelDefinition", func() {
	var shovel *rabbitmqv1beta1.Shovel

	BeforeEach(func() {
		shovel = &rabbitmqv1beta1.Shovel{
			ObjectMeta: metav1.ObjectMeta{
				Name: "new-shovel",
			},
			Spec: rabbitmqv1beta1.ShovelSpec{
				Vhost: "/new-vhost",
				Name:  "new-shovel",
			},
		}
	})

	It("sets source and destination uris correctly for a single uri", func() {
		definition := GenerateShovelDefinition(shovel, "a-rabbitmq-src@test.com", "a-rabbitmq-dest@test.com")
		Expect(definition.SourceURI).To(ConsistOf("a-rabbitmq-src@test.com"))
		Expect(definition.DestinationURI).To(ConsistOf("a-rabbitmq-dest@test.com"))
	})

	It("sets source and destination uris correctly for multiple uris", func() {
		definition := GenerateShovelDefinition(shovel, "a-rabbitmq-src@test.com0,a-rabbitmq-src@test1.com", "a-rabbitmq-dest@test0.com,a-rabbitmq-dest@test1.com")
		Expect(definition.SourceURI).To(ConsistOf("a-rabbitmq-src@test.com0", "a-rabbitmq-src@test1.com"))
		Expect(definition.DestinationURI).To(ConsistOf("a-rabbitmq-dest@test0.com", "a-rabbitmq-dest@test1.com"))
	})

	It("sets 'AckMode' correctly", func() {
		shovel.Spec.AckMode = "on-publish"
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(definition.AckMode).To(Equal("on-publish"))
	})

	It("sets 'AddForwardHeaders' correctly", func() {
		shovel.Spec.AddForwardHeaders = true
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(definition.AddForwardHeaders).To(BeTrue())
	})

	It("sets 'DeleteAfter' correctly", func() {
		shovel.Spec.DeleteAfter = "never"
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(string(definition.DeleteAfter)).To(Equal("never"))
	})

	It("sets 'DestinationAddForwardHeaders' correctly", func() {
		shovel.Spec.DestinationAddForwardHeaders = true
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(definition.DestinationAddForwardHeaders).To(BeTrue())
	})

	It("sets 'DestinationAddTimestampHeader' correctly", func() {
		shovel.Spec.DestinationAddTimestampHeader = true
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(definition.DestinationAddTimestampHeader).To(BeTrue())
	})

	It("sets 'DestinationAddress' correctly", func() {
		shovel.Spec.DestinationAddress = "an-address"
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(definition.DestinationAddress).To(Equal("an-address"))
	})

	It("sets 'DestinationApplicationProperties' correctly", func() {
		shovel.Spec.DestinationApplicationProperties = "a-property"
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(definition.DestinationApplicationProperties).To(Equal("a-property"))
	})

	It("sets 'DestinationExchange' correctly", func() {
		shovel.Spec.DestinationExchange = "an-exchange"
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(definition.DestinationExchange).To(Equal("an-exchange"))
	})

	It("sets 'DestinationExchangeKey' correctly", func() {
		shovel.Spec.DestinationExchangeKey = "a-key"
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(definition.DestinationExchangeKey).To(Equal("a-key"))
	})

	It("sets 'DestinationProperties' correctly", func() {
		shovel.Spec.DestinationProperties = "a-property"
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(definition.DestinationProperties).To(Equal("a-property"))
	})

	It("sets 'DestinationProtocol' correctly", func() {
		shovel.Spec.DestinationProtocol = "amqp10"
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(definition.DestinationProtocol).To(Equal("amqp10"))
	})

	It("sets 'DestinationPublishProperties' correctly", func() {
		shovel.Spec.DestinationPublishProperties = "a-publish-property"
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(definition.DestinationPublishProperties).To(Equal("a-publish-property"))
	})

	It("sets 'DestinationQueue' correctly", func() {
		shovel.Spec.DestinationQueue = "a-destination-queue"
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(definition.DestinationQueue).To(Equal("a-destination-queue"))
	})

	It("sets 'PrefetchCount' correctly", func() {
		shovel.Spec.PrefetchCount = 200
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(definition.PrefetchCount).To(Equal(200))
	})

	It("sets 'ReconnectDelay' correctly", func() {
		shovel.Spec.ReconnectDelay = 2000
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(definition.ReconnectDelay).To(Equal(2000))
	})

	It("sets 'SourceAddress' correctly", func() {
		shovel.Spec.SourceAddress = "an-address"
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(definition.SourceAddress).To(Equal("an-address"))
	})

	It("sets 'SourceDeleteAfter' correctly", func() {
		shovel.Spec.SourceDeleteAfter = "10000000"
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(string(definition.SourceDeleteAfter)).To(Equal("10000000"))
	})

	It("sets 'SourceExchange' correctly", func() {
		shovel.Spec.SourceExchange = "an-source-exchange"
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(definition.SourceExchange).To(Equal("an-source-exchange"))
	})

	It("sets 'SourceExchangeKey' correctly", func() {
		shovel.Spec.SourceExchangeKey = "an-source-exchange-key"
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(definition.SourceExchangeKey).To(Equal("an-source-exchange-key"))
	})

	It("sets 'SourcePrefetchCount' correctly", func() {
		shovel.Spec.SourcePrefetchCount = 200
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(definition.SourcePrefetchCount).To(Equal(200))
	})

	It("sets 'SourceProtocol' correctly", func() {
		shovel.Spec.SourceProtocol = "amqp09"
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(definition.SourceProtocol).To(Equal("amqp09"))
	})

	It("sets 'SourceQueue' correctly", func() {
		shovel.Spec.SourceQueue = "a-great-queue"
		definition := GenerateShovelDefinition(shovel, "", "")
		Expect(definition.SourceQueue).To(Equal("a-great-queue"))
	})
})
