package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("user webhook", func() {
	var user = User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: UserSpec{
			Tags: []UserTag{"policymaker"},
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name: "a-cluster",
			},
		},
	}

	Context("ValidateCreate", func() {
		It("does not allow both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret be configured", func() {
			notAllowed := user.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = &corev1.LocalObjectReference{Name: "some-secret"}
			Expect(apierrors.IsForbidden(notAllowed.ValidateCreate())).To(BeTrue())
		})

		It("spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret cannot both be empty", func() {
			notAllowed := user.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.Name = ""
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			Expect(apierrors.IsForbidden(notAllowed.ValidateCreate())).To(BeTrue())
		})
	})

	Context("ValidateUpdate", func() {
		It("does not allow updates on RabbitmqClusterReference", func() {
			newUser := user.DeepCopy()
			newUser.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
				Name: "newUser-cluster",
			}
			Expect(apierrors.IsForbidden(newUser.ValidateUpdate(&user))).To(BeTrue())
		})

		It("does not allow updates on rabbitmqClusterReference.connectionSecret", func() {
			connectionScr := User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: UserSpec{
					Tags: []UserTag{"policymaker"},
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

		It("allows update on tags", func() {
			newUser := user.DeepCopy()
			newUser.Spec.Tags = []UserTag{"monitoring"}
			Expect(newUser.ValidateUpdate(&user)).To(Succeed())
		})
	})
})
