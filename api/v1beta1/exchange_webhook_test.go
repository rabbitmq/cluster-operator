package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("exchange webhook", func() {

	var exchange = Exchange{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-exchange",
		},
		Spec: ExchangeSpec{
			Name:       "test",
			Vhost:      "/test",
			Type:       "fanout",
			Durable:    false,
			AutoDelete: true,
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name: "some-cluster",
			},
		},
	}

	Context("ValidateCreate", func() {
		It("does not allow both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret be configured", func() {
			notAllowed := exchange.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = &corev1.LocalObjectReference{Name: "some-secret"}
			Expect(apierrors.IsForbidden(notAllowed.ValidateCreate())).To(BeTrue())
		})

		It("spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret cannot both be empty", func() {
			notAllowed := exchange.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.Name = ""
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			Expect(apierrors.IsForbidden(notAllowed.ValidateCreate())).To(BeTrue())
		})
	})

	Context("ValidateUpdate", func() {
		It("does not allow updates on exchange name", func() {
			newExchange := exchange.DeepCopy()
			newExchange.Spec.Name = "new-name"
			Expect(apierrors.IsForbidden(newExchange.ValidateUpdate(&exchange))).To(BeTrue())
		})

		It("does not allow updates on vhost", func() {
			newExchange := exchange.DeepCopy()
			newExchange.Spec.Vhost = "/a-new-vhost"
			Expect(apierrors.IsForbidden(newExchange.ValidateUpdate(&exchange))).To(BeTrue())
		})

		It("does not allow updates on RabbitmqClusterReference", func() {
			newExchange := exchange.DeepCopy()
			newExchange.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
				Name: "new-cluster",
			}
			Expect(apierrors.IsForbidden(newExchange.ValidateUpdate(&exchange))).To(BeTrue())
		})

		It("does not allow updates on rabbitmqClusterReference.connectionSecret", func() {
			connectionScr := Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-exchange",
				},
				Spec: ExchangeSpec{
					Name:  "test",
					Vhost: "/test",
					Type:  "fanout",
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

		It("does not allow updates on exchange type", func() {
			newExchange := exchange.DeepCopy()
			newExchange.Spec.Type = "direct"
			Expect(apierrors.IsInvalid(newExchange.ValidateUpdate(&exchange))).To(BeTrue())
		})

		It("does not allow updates on durable", func() {
			newExchange := exchange.DeepCopy()
			newExchange.Spec.Durable = true
			Expect(apierrors.IsInvalid(newExchange.ValidateUpdate(&exchange))).To(BeTrue())
		})

		It("does not allow updates on autoDelete", func() {
			newExchange := exchange.DeepCopy()
			newExchange.Spec.AutoDelete = false
			Expect(apierrors.IsInvalid(newExchange.ValidateUpdate(&exchange))).To(BeTrue())
		})

		It("allows updates on arguments", func() {
			newExchange := exchange.DeepCopy()
			newExchange.Spec.Arguments = &runtime.RawExtension{Raw: []byte(`{"new":"new-value"}`)}
			Expect(newExchange.ValidateUpdate(&exchange)).To(Succeed())
		})
	})
})
