package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Binding webhook", func() {

	var oldBinding = Binding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "update-binding",
		},
		Spec: BindingSpec{
			Vhost:           "/test",
			Source:          "test",
			Destination:     "test",
			DestinationType: "queue",
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name: "some-cluster",
			},
		},
	}

	Context("ValidateCreate", func() {
		It("does not allow both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret be configured", func() {
			notAllowed := oldBinding.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = &corev1.LocalObjectReference{Name: "some-secret"}
			Expect(apierrors.IsForbidden(notAllowed.ValidateCreate())).To(BeTrue())
		})

		It("spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret cannot both be empty", func() {
			notAllowed := oldBinding.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.Name = ""
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			Expect(apierrors.IsForbidden(notAllowed.ValidateCreate())).To(BeTrue())
		})
	})

	Context("ValidateUpdate", func() {
		It("does not allow updates on vhost", func() {
			newBinding := oldBinding.DeepCopy()
			newBinding.Spec.Vhost = "/new-vhost"
			Expect(apierrors.IsForbidden(newBinding.ValidateUpdate(&oldBinding))).To(BeTrue())
		})

		It("does not allow updates on RabbitmqClusterReference", func() {
			newBinding := oldBinding.DeepCopy()
			newBinding.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
				Name: "new-cluster",
			}
			Expect(apierrors.IsForbidden(newBinding.ValidateUpdate(&oldBinding))).To(BeTrue())
		})

		It("does not allow updates on rabbitmqClusterReference.connectionSecret", func() {
			connectionScr := Binding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "connect-test-queue",
				},
				Spec: BindingSpec{
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

		It("does not allow updates on source", func() {
			newBinding := oldBinding.DeepCopy()
			newBinding.Spec.Source = "updated-source"
			Expect(apierrors.IsInvalid(newBinding.ValidateUpdate(&oldBinding))).To(BeTrue())
		})

		It("does not allow updates on destination", func() {
			newBinding := oldBinding.DeepCopy()
			newBinding.Spec.Destination = "updated-des"
			Expect(apierrors.IsInvalid(newBinding.ValidateUpdate(&oldBinding))).To(BeTrue())
		})

		It("does not allow updates on destination type", func() {
			newBinding := oldBinding.DeepCopy()
			newBinding.Spec.DestinationType = "exchange"
			Expect(apierrors.IsInvalid(newBinding.ValidateUpdate(&oldBinding))).To(BeTrue())
		})

		It("does not allow updates on routing key", func() {
			newBinding := oldBinding.DeepCopy()
			newBinding.Spec.RoutingKey = "not-allowed"
			Expect(apierrors.IsInvalid(newBinding.ValidateUpdate(&oldBinding))).To(BeTrue())
		})

		It("does not allow updates on binding arguments", func() {
			newBinding := oldBinding.DeepCopy()
			newBinding.Spec.Arguments = &runtime.RawExtension{Raw: []byte(`{"new":"new-value"}`)}
			Expect(apierrors.IsInvalid(newBinding.ValidateUpdate(&oldBinding))).To(BeTrue())
		})
	})

})
