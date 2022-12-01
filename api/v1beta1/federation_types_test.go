package v1beta1

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Federation spec", func() {
	var (
		namespace = "default"
		ctx       = context.Background()
	)

	It("creates a federation with minimal configurations", func() {
		expectedSpec := FederationSpec{
			Name:  "test-federation",
			Vhost: "/",
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name: "some-cluster",
			},
			UriSecret: &corev1.LocalObjectReference{
				Name: "a-secret",
			},
		}

		federation := Federation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-federation",
				Namespace: namespace,
			},
			Spec: FederationSpec{
				Name: "test-federation",
				UriSecret: &corev1.LocalObjectReference{
					Name: "a-secret",
				},
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "some-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &federation)).To(Succeed())
		fetched := &Federation{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      federation.Name,
			Namespace: federation.Namespace,
		}, fetched)).To(Succeed())
		Expect(fetched.Spec).To(Equal(expectedSpec))
	})

	It("creates a federation with configurations", func() {
		federation := Federation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configured-federation",
				Namespace: namespace,
			},
			Spec: FederationSpec{
				Name:  "configured-federation",
				Vhost: "/hello",
				UriSecret: &corev1.LocalObjectReference{
					Name: "a-secret",
				},
				Expires:        1000,
				MessageTTL:     1000,
				MaxHops:        100,
				PrefetchCount:  50,
				ReconnectDelay: 10,
				TrustUserId:    true,
				Exchange:       "an-exchange",
				AckMode:        "no-ack",
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "some-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &federation)).To(Succeed())
		fetched := &Federation{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      federation.Name,
			Namespace: federation.Namespace,
		}, fetched)).To(Succeed())

		Expect(fetched.Spec.Name).To(Equal("configured-federation"))
		Expect(fetched.Spec.Vhost).To(Equal("/hello"))
		Expect(fetched.Spec.RabbitmqClusterReference).To(Equal(
			RabbitmqClusterReference{
				Name: "some-cluster",
			}))

		Expect(fetched.Spec.UriSecret.Name).To(Equal("a-secret"))
		Expect(fetched.Spec.AckMode).To(Equal("no-ack"))
		Expect(fetched.Spec.Exchange).To(Equal("an-exchange"))
		Expect(fetched.Spec.Expires).To(Equal(1000))
		Expect(fetched.Spec.MessageTTL).To(Equal(1000))
		Expect(fetched.Spec.MaxHops).To(Equal(100))
		Expect(fetched.Spec.PrefetchCount).To(Equal(50))
		Expect(fetched.Spec.ReconnectDelay).To(Equal(10))
	})

	When("creating a federation with an invalid 'AckMode' value", func() {
		It("fails with validation errors", func() {
			federation := Federation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-federation",
					Namespace: namespace,
				},
				Spec: FederationSpec{
					Name: "test-federation",
					UriSecret: &corev1.LocalObjectReference{
						Name: "a-secret",
					},
					AckMode: "non-existing-ackmode",
					RabbitmqClusterReference: RabbitmqClusterReference{
						Name: "some-cluster",
					},
				},
			}
			Expect(k8sClient.Create(ctx, &federation)).To(HaveOccurred())
			Expect(k8sClient.Create(ctx, &federation)).To(MatchError(`Federation.rabbitmq.com "invalid-federation" is invalid: spec.ackMode: Unsupported value: "non-existing-ackmode": supported values: "on-confirm", "on-publish", "no-ack"`))
		})
	})
})
