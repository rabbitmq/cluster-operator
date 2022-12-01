package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("vhost webhook", func() {

	var vhost = Vhost{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-vhost",
		},
		Spec: VhostSpec{
			Name:    "test",
			Tracing: false,
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name: "a-cluster",
			},
		},
	}

	Context("ValidateCreate", func() {
		It("does not allow both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret be configured", func() {
			notAllowed := vhost.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = &corev1.LocalObjectReference{Name: "some-secret"}
			Expect(apierrors.IsForbidden(notAllowed.ValidateCreate())).To(BeTrue())
		})

		It("spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret cannot both be empty", func() {
			notAllowed := vhost.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.Name = ""
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			Expect(apierrors.IsForbidden(notAllowed.ValidateCreate())).To(BeTrue())
		})
	})

	Context("ValidateUpdate", func() {
		It("does not allow updates on vhost name", func() {
			newVhost := vhost.DeepCopy()
			newVhost.Spec.Name = "new-name"
			Expect(apierrors.IsForbidden(newVhost.ValidateUpdate(&vhost))).To(BeTrue())
		})

		It("does not allow updates on RabbitmqClusterReference", func() {
			newVhost := vhost.DeepCopy()
			newVhost.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
				Name: "new-cluster",
			}
			Expect(apierrors.IsForbidden(newVhost.ValidateUpdate(&vhost))).To(BeTrue())
		})

		It("does not allow updates on rabbitmqClusterReference.connectionSecret", func() {
			connectionScr := Vhost{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vhost",
				},
				Spec: VhostSpec{
					Name: "test",
					RabbitmqClusterReference: RabbitmqClusterReference{
						ConnectionSecret: &corev1.LocalObjectReference{
							Name: "a-secret",
						},
					},
				},
			}
			new := connectionScr.DeepCopy()
			new.Spec.RabbitmqClusterReference.Name = "a-name"
			new.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			Expect(apierrors.IsForbidden(new.ValidateUpdate(&connectionScr))).To(BeTrue())
		})

		It("allows updates on vhost.spec.tracing", func() {
			newVhost := vhost.DeepCopy()
			newVhost.Spec.Tracing = true
			Expect(newVhost.ValidateUpdate(&vhost)).To(Succeed())
		})

		It("allows updates on vhost.spec.tags", func() {
			newVhost := vhost.DeepCopy()
			newVhost.Spec.Tags = []string{"new-tag"}
			Expect(newVhost.ValidateUpdate(&vhost)).To(Succeed())
		})
	})
})
