package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("topic permission webhook", func() {
	var permission = TopicPermission{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: TopicPermissionSpec{
			User:  "test-user",
			Vhost: "/a-vhost",
			Permissions: TopicPermissionConfig{
				Exchange: "a-exchange",
				Read:     ".*",
				Write:    ".*",
			},
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name: "a-cluster",
			},
		},
	}

	Context("ValidateCreate", func() {
		It("does not allow user and userReference to be specified at the same time", func() {
			invalidPermission := permission.DeepCopy()
			invalidPermission.Spec.UserReference = &corev1.LocalObjectReference{Name: "invalid"}
			invalidPermission.Spec.User = "test-user"
			Expect(apierrors.IsInvalid(invalidPermission.ValidateCreate())).To(BeTrue())
		})
		It("does not allow both user and userReference to be unset", func() {
			invalidPermission := permission.DeepCopy()
			invalidPermission.Spec.UserReference = nil
			invalidPermission.Spec.User = ""
			Expect(apierrors.IsInvalid(invalidPermission.ValidateCreate())).To(BeTrue())
		})

		It("does not allow both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret be configured", func() {
			notAllowed := permission.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = &corev1.LocalObjectReference{Name: "some-secret"}
			Expect(apierrors.IsForbidden(notAllowed.ValidateCreate())).To(BeTrue())
		})

		It("spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret cannot both be empty", func() {
			notAllowed := permission.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.Name = ""
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			Expect(apierrors.IsForbidden(notAllowed.ValidateCreate())).To(BeTrue())
		})
	})

	Context("ValidateUpdate", func() {
		It("does not allow updates on user", func() {
			newPermission := permission.DeepCopy()
			newPermission.Spec.User = "new-user"
			Expect(apierrors.IsForbidden(newPermission.ValidateUpdate(&permission))).To(BeTrue())
		})

		It("does not allow updates on userReference", func() {
			permissionWithUserRef := permission.DeepCopy()
			permissionWithUserRef.Spec.User = ""
			permissionWithUserRef.Spec.UserReference = &corev1.LocalObjectReference{Name: "a-user"}
			newPermission := permissionWithUserRef.DeepCopy()
			newPermission.Spec.UserReference = &corev1.LocalObjectReference{Name: "a-new-user"}
			Expect(apierrors.IsForbidden(newPermission.ValidateUpdate(permissionWithUserRef))).To(BeTrue())
		})

		It("does not allow updates on vhost", func() {
			newPermission := permission.DeepCopy()
			newPermission.Spec.Vhost = "new-vhost"
			Expect(apierrors.IsForbidden(newPermission.ValidateUpdate(&permission))).To(BeTrue())
		})

		It("does not allow updates on RabbitmqClusterReference", func() {
			newPermission := permission.DeepCopy()
			newPermission.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
				Name: "new-cluster",
			}
			Expect(apierrors.IsForbidden(newPermission.ValidateUpdate(&permission))).To(BeTrue())
		})

		It("does not allow updates on rabbitmqClusterReference.connectionSecret", func() {
			connectionScr := Permission{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: PermissionSpec{
					User:  "test-user",
					Vhost: "/a-vhost",
					Permissions: VhostPermissions{
						Configure: ".*",
						Read:      ".*",
						Write:     ".*",
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

		It("does not allow updates on spec.permissions.exchange", func() {
			newPermission := permission.DeepCopy()
			newPermission.Spec.Permissions.Exchange = "a-different-exchange"
			Expect(apierrors.IsForbidden(newPermission.ValidateUpdate(&permission))).To(BeTrue())
		})

		It("allows updates on permission.spec.permissions.read", func() {
			newPermission := permission.DeepCopy()
			newPermission.Spec.Permissions.Read = "?"
			Expect(newPermission.ValidateUpdate(&permission)).To(Succeed())
		})

		It("allows updates on permission.spec.permissions.write", func() {
			newPermission := permission.DeepCopy()
			newPermission.Spec.Permissions.Write = "?"
			Expect(newPermission.ValidateUpdate(&permission)).To(Succeed())
		})

		It("does not allow user and userReference to be specified at the same time", func() {
			newPermission := permission.DeepCopy()
			newPermission.Spec.UserReference = &corev1.LocalObjectReference{Name: "invalid"}
			Expect(apierrors.IsInvalid(newPermission.ValidateUpdate(&permission))).To(BeTrue())
		})

		It("does not allow both user and userReference to be unset", func() {
			newPermission := permission.DeepCopy()
			newPermission.Spec.User = ""
			newPermission.Spec.UserReference = nil
			Expect(apierrors.IsInvalid(newPermission.ValidateUpdate(&permission))).To(BeTrue())
		})
	})
})
