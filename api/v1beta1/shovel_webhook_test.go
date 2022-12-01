package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("shovel webhook", func() {
	var shovel = Shovel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: ShovelSpec{
			Name:  "test-upstream",
			Vhost: "/a-vhost",
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
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name: "a-cluster",
			},
		},
	}

	Context("ValidateCreate", func() {
		It("does not allow both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret be configured", func() {
			notAllowed := shovel.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = &corev1.LocalObjectReference{Name: "some-secret"}
			Expect(apierrors.IsForbidden(notAllowed.ValidateCreate())).To(BeTrue())
		})

		It("spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret cannot both be empty", func() {
			notAllowed := shovel.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.Name = ""
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			Expect(apierrors.IsForbidden(notAllowed.ValidateCreate())).To(BeTrue())
		})
	})

	Context("ValidateUpdate", func() {
		It("does not allow updates on name", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.Name = "another-shovel"
			Expect(apierrors.IsForbidden(newShovel.ValidateUpdate(&shovel))).To(BeTrue())
		})

		It("does not allow updates on vhost", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.Vhost = "another-vhost"
			Expect(apierrors.IsForbidden(newShovel.ValidateUpdate(&shovel))).To(BeTrue())
		})

		It("does not allow updates on RabbitmqClusterReference", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
				Name: "another-cluster",
			}
			Expect(apierrors.IsForbidden(newShovel.ValidateUpdate(&shovel))).To(BeTrue())
		})

		It("does not allow updates on rabbitmqClusterReference.connectionSecret", func() {
			connectionScr := Shovel{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: ShovelSpec{
					Name:  "test-upstream",
					Vhost: "/a-vhost",
					UriSecret: &corev1.LocalObjectReference{
						Name: "a-secret",
					},
					RabbitmqClusterReference: RabbitmqClusterReference{
						ConnectionSecret: &corev1.LocalObjectReference{
							Name: "a-secret",
						},
					},
				},
			}
			new := connectionScr.DeepCopy()
			new.Spec.RabbitmqClusterReference.ConnectionSecret.Name = "new-secret"
			Expect(apierrors.IsForbidden(new.ValidateUpdate(&connectionScr))).To(BeTrue())
		})

		It("allows updates on shovel.spec.uriSecret", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.UriSecret = &corev1.LocalObjectReference{Name: "another-secret"}
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on AckMode", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.AckMode = "on-confirm"
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on AddForwardHeaders", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.AddForwardHeaders = false
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on DeleteAfter", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.DeleteAfter = "100"
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on AddForwardHeaders", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.AddForwardHeaders = false
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on DestinationAddForwardHeaders", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.DestinationAddForwardHeaders = false
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on DestinationAddTimestampHeader", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.DestinationAddTimestampHeader = false
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on DestinationAddress", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.DeleteAfter = "another"
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on DestinationApplicationProperties", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.DestinationApplicationProperties = "new-property"
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on DestinationExchange", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.DestinationExchange = "new-exchange"
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on DestinationExchangeKey", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.DestinationExchangeKey = "new-key"
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on DestinationProperties", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.DestinationProperties = "new"
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})
		It("allows updates on DestinationProtocol", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.DestinationProtocol = "new"
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on DestinationPublishProperties", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.DestinationPublishProperties = "new-property"
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on DestinationQueue", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.DestinationQueue = "another-queue"
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on shovel.spec.PrefetchCount", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.PrefetchCount = 20
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on shovel.spec.reconnectDelay", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.PrefetchCount = 10000
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on SourceAddress", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.SourceAddress = "another-queue"
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on SourceDeleteAfter", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.SourceDeleteAfter = "100"
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on SourceExchange", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.SourceExchange = "another-exchange"
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on SourceExchangeKey", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.SourceExchangeKey = "another-key"
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on SourcePrefetchCount", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.SourcePrefetchCount = 50
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on SourceProtocol", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.SourceProtocol = "another-protocol"
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})

		It("allows updates on SourceQueue", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.SourceQueue = "another-queue"
			Expect(newShovel.ValidateUpdate(&shovel)).To(Succeed())
		})
	})
})
