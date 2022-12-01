package v1beta1

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Shovel spec", func() {
	var (
		namespace = "default"
		ctx       = context.Background()
	)

	It("creates a shovel with minimal configurations", func() {
		shovel := Shovel{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-shovel",
				Namespace: namespace,
			},
			Spec: ShovelSpec{
				Name: "test-shovel",
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "some-cluster",
				},
				UriSecret: &corev1.LocalObjectReference{
					Name: "a-secret",
				},
			}}
		Expect(k8sClient.Create(ctx, &shovel)).To(Succeed())
		fetched := &Shovel{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      shovel.Name,
			Namespace: shovel.Namespace,
		}, fetched)).To(Succeed())
		Expect(fetched.Spec.Name).To(Equal("test-shovel"))
		Expect(fetched.Spec.Vhost).To(Equal("/"))
		Expect(fetched.Spec.RabbitmqClusterReference.Name).To(Equal("some-cluster"))
		Expect(fetched.Spec.UriSecret.Name).To(Equal("a-secret"))
	})

	It("creates shovel with configurations", func() {
		shovel := Shovel{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-shovel-configurations",
				Namespace: namespace,
			},
			Spec: ShovelSpec{
				Name:  "test-shovel-configurations",
				Vhost: "test-vhost",
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "some-cluster",
				},
				UriSecret: &corev1.LocalObjectReference{
					Name: "a-secret",
				},
				AckMode:                          "no-ack",
				AddForwardHeaders:                true,
				DeleteAfter:                      "never",
				DestinationAddForwardHeaders:     true,
				DestinationAddTimestampHeader:    true,
				DestinationAddress:               "myQueue",
				DestinationApplicationProperties: "a-property",
				DestinationExchange:              "an-exchange",
				DestinationExchangeKey:           "a-key",
				DestinationProperties:            "a-property",
				DestinationProtocol:              "amqp091",
				DestinationPublishProperties:     "a-property",
				DestinationQueue:                 "a-queue",
				PrefetchCount:                    10,
				ReconnectDelay:                   10,
				SourceAddress:                    "myQueue",
				SourceDeleteAfter:                "never",
				SourceExchange:                   "an-exchange",
				SourceExchangeKey:                "a-key",
				SourcePrefetchCount:              10,
				SourceProtocol:                   "amqp091",
				SourceQueue:                      "a-queue",
			}}
		Expect(k8sClient.Create(ctx, &shovel)).To(Succeed())
		fetched := &Shovel{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      shovel.Name,
			Namespace: shovel.Namespace,
		}, fetched)).To(Succeed())

		Expect(fetched.Spec.Name).To(Equal("test-shovel-configurations"))
		Expect(fetched.Spec.Vhost).To(Equal("test-vhost"))
		Expect(fetched.Spec.RabbitmqClusterReference.Name).To(Equal("some-cluster"))
		Expect(fetched.Spec.UriSecret.Name).To(Equal("a-secret"))
		Expect(fetched.Spec.AckMode).To(Equal("no-ack"))
		Expect(fetched.Spec.AddForwardHeaders).To(BeTrue())
		Expect(fetched.Spec.DeleteAfter).To(Equal("never"))

		Expect(fetched.Spec.DestinationAddTimestampHeader).To(BeTrue())
		Expect(fetched.Spec.DestinationAddForwardHeaders).To(BeTrue())
		Expect(fetched.Spec.DestinationAddress).To(Equal("myQueue"))
		Expect(fetched.Spec.DestinationApplicationProperties).To(Equal("a-property"))
		Expect(fetched.Spec.DestinationExchange).To(Equal("an-exchange"))
		Expect(fetched.Spec.DestinationExchangeKey).To(Equal("a-key"))
		Expect(fetched.Spec.DestinationProperties).To(Equal("a-property"))
		Expect(fetched.Spec.DestinationQueue).To(Equal("a-queue"))
		Expect(fetched.Spec.PrefetchCount).To(Equal(10))
		Expect(fetched.Spec.ReconnectDelay).To(Equal(10))

		Expect(fetched.Spec.SourceAddress).To(Equal("myQueue"))
		Expect(fetched.Spec.SourceDeleteAfter).To(Equal("never"))
		Expect(fetched.Spec.SourceExchange).To(Equal("an-exchange"))
		Expect(fetched.Spec.SourceExchangeKey).To(Equal("a-key"))
		Expect(fetched.Spec.SourcePrefetchCount).To(Equal(10))
		Expect(fetched.Spec.SourceProtocol).To(Equal("amqp091"))
		Expect(fetched.Spec.SourceQueue).To(Equal("a-queue"))
	})

	When("creating a shovel with an invalid 'AckMode' value", func() {
		It("fails with validation errors", func() {
			shovel := Shovel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "an-invalid-ackmode",
					Namespace: namespace,
				},
				Spec: ShovelSpec{
					Name: "an-invalid-ackmode",
					RabbitmqClusterReference: RabbitmqClusterReference{
						Name: "some-cluster",
					},
					UriSecret: &corev1.LocalObjectReference{
						Name: "a-secret",
					},
					AckMode: "an-invalid-ackmode",
				}}
			Expect(k8sClient.Create(ctx, &shovel)).To(HaveOccurred())
			Expect(k8sClient.Create(ctx, &shovel)).To(MatchError(`Shovel.rabbitmq.com "an-invalid-ackmode" is invalid: spec.ackMode: Unsupported value: "an-invalid-ackmode": supported values: "on-confirm", "on-publish", "no-ack"`))
		})
	})
})
