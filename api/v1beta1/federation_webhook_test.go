package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("federation webhook", func() {
	var federation = Federation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: FederationSpec{
			Name:  "test-upstream",
			Vhost: "/a-vhost",
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
				Name: "a-cluster",
			},
		},
	}
	Context("ValidateCreate", func() {
		It("does not allow both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret be configured", func() {
			notAllowed := federation.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = &corev1.LocalObjectReference{Name: "some-secret"}
			Expect(apierrors.IsForbidden(notAllowed.ValidateCreate())).To(BeTrue())
		})

		It("spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret cannot both be empty", func() {
			notAllowed := federation.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.Name = ""
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			Expect(apierrors.IsForbidden(notAllowed.ValidateCreate())).To(BeTrue())
		})
	})

	Context("ValidateUpdate", func() {
		It("does not allow updates on name", func() {
			newFederation := federation.DeepCopy()
			newFederation.Spec.Name = "new-upstream"
			Expect(apierrors.IsForbidden(newFederation.ValidateUpdate(&federation))).To(BeTrue())
		})

		It("does not allow updates on vhost", func() {
			newFederation := federation.DeepCopy()
			newFederation.Spec.Vhost = "new-vhost"
			Expect(apierrors.IsForbidden(newFederation.ValidateUpdate(&federation))).To(BeTrue())
		})

		It("does not allow updates on RabbitmqClusterReference", func() {
			newFederation := federation.DeepCopy()
			newFederation.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
				Name: "new-cluster",
			}
			Expect(apierrors.IsForbidden(newFederation.ValidateUpdate(&federation))).To(BeTrue())
		})

		It("does not allow updates on rabbitmqClusterReference.connectionSecret", func() {
			connectionScr := Federation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: FederationSpec{
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

		It("allows updates on federation.spec.uriSecret", func() {
			newFederation := federation.DeepCopy()
			newFederation.Spec.UriSecret = &corev1.LocalObjectReference{Name: "a-new-secret"}
			Expect(newFederation.ValidateUpdate(&federation)).To(Succeed())
		})

		It("allows updates on federation.spec.expires", func() {
			newFederation := federation.DeepCopy()
			newFederation.Spec.Expires = 10
			Expect(newFederation.ValidateUpdate(&federation)).To(Succeed())
		})

		It("allows updates on federation.spec.messageTTL", func() {
			newFederation := federation.DeepCopy()
			newFederation.Spec.MessageTTL = 10
			Expect(newFederation.ValidateUpdate(&federation)).To(Succeed())
		})

		It("allows updates on federation.spec.maxHops", func() {
			newFederation := federation.DeepCopy()
			newFederation.Spec.MaxHops = 10
			Expect(newFederation.ValidateUpdate(&federation)).To(Succeed())
		})

		It("allows updates on federation.spec.prefetchCount", func() {
			newFederation := federation.DeepCopy()
			newFederation.Spec.PrefetchCount = 10
			Expect(newFederation.ValidateUpdate(&federation)).To(Succeed())
		})

		It("allows updates on federation.spec.reconnectDelay", func() {
			newFederation := federation.DeepCopy()
			newFederation.Spec.PrefetchCount = 10000
			Expect(newFederation.ValidateUpdate(&federation)).To(Succeed())
		})

		It("allows updates on federation.spec.trustUserId", func() {
			newFederation := federation.DeepCopy()
			newFederation.Spec.TrustUserId = false
			Expect(newFederation.ValidateUpdate(&federation)).To(Succeed())
		})

		It("allows updates on federation.spec.exchange", func() {
			newFederation := federation.DeepCopy()
			newFederation.Spec.Exchange = "new-exchange"
			Expect(newFederation.ValidateUpdate(&federation)).To(Succeed())
		})

		It("allows updates on federation.spec.ackMode", func() {
			newFederation := federation.DeepCopy()
			newFederation.Spec.AckMode = "no-ack"
			Expect(newFederation.ValidateUpdate(&federation)).To(Succeed())
		})
	})
})
