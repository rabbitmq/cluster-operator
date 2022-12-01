package system_tests

import (
	"context"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Permission", func() {
	var (
		namespace  = MustHaveEnv("NAMESPACE")
		ctx        = context.Background()
		permission *rabbitmqv1beta1.Permission
		user       *rabbitmqv1beta1.User
		username   string
	)

	BeforeEach(func() {
		user = &rabbitmqv1beta1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testuser",
				Namespace: namespace,
			},
			Spec: rabbitmqv1beta1.UserSpec{
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: rmq.Name,
				},
				Tags: []rabbitmqv1beta1.UserTag{"management"},
			},
		}
		Expect(rmqClusterClient.Create(ctx, user, &client.CreateOptions{})).To(Succeed())
		generatedSecretKey := types.NamespacedName{
			Name:      "testuser-user-credentials",
			Namespace: namespace,
		}
		var generatedSecret = &corev1.Secret{}
		Eventually(func() error {
			return rmqClusterClient.Get(ctx, generatedSecretKey, generatedSecret)
		}, 30, 2).Should(Succeed())
		username = string(generatedSecret.Data["username"])

		permission = &rabbitmqv1beta1.Permission{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "user-permission",
				Namespace: namespace,
			},
			Spec: rabbitmqv1beta1.PermissionSpec{
				Vhost: "/",
				User:  username,
				Permissions: rabbitmqv1beta1.VhostPermissions{
					Configure: ".*",
					Read:      ".*",
				},
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: rmq.Name,
				},
			},
		}
	})

	AfterEach(func() {
		Expect(rmqClusterClient.Delete(ctx, user)).To(Succeed())
		Eventually(func() string {
			if err := rmqClusterClient.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &rabbitmqv1beta1.User{}); err != nil {
				return err.Error()
			}
			return ""
		}, 10).Should(ContainSubstring("not found"))
	})

	DescribeTable("Server configurations updates", func(testcase string) {
		if testcase == "UserReference" {
			permission.Spec.User = ""
			permission.Spec.UserReference = &corev1.LocalObjectReference{Name: user.Name}
		}
		Expect(rmqClusterClient.Create(ctx, permission, &client.CreateOptions{})).To(Succeed())
		var fetchedPermissionInfo rabbithole.PermissionInfo
		Eventually(func() error {
			var err error
			fetchedPermissionInfo, err = rabbitClient.GetPermissionsIn(permission.Spec.Vhost, username)
			return err
		}, 30, 2).Should(Not(HaveOccurred()))
		Expect(fetchedPermissionInfo).To(MatchFields(IgnoreExtras, Fields{
			"Vhost":     Equal(permission.Spec.Vhost),
			"User":      Equal(username),
			"Configure": Equal(permission.Spec.Permissions.Configure),
			"Read":      Equal(permission.Spec.Permissions.Read),
			"Write":     Equal(permission.Spec.Permissions.Write),
		}))

		By("updating status condition 'Ready'")
		updatedPermission := rabbitmqv1beta1.Permission{}

		Eventually(func() []rabbitmqv1beta1.Condition {
			Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &updatedPermission)).To(Succeed())
			return updatedPermission.Status.Conditions
		}, waitUpdatedStatusCondition, 2).Should(HaveLen(1), "Permission status condition should be present")

		readyCondition := updatedPermission.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
		Expect(readyCondition.LastTransitionTime).NotTo(Equal(metav1.Time{}))

		By("setting status.observedGeneration")
		Expect(updatedPermission.Status.ObservedGeneration).To(Equal(updatedPermission.GetGeneration()))

		By("not allowing updates on certain fields")
		updateTest := rabbitmqv1beta1.Permission{}
		Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &updateTest)).To(Succeed())
		updateTest.Spec.Vhost = "/a-new-vhost"
		Expect(rmqClusterClient.Update(ctx, &updateTest).Error()).To(ContainSubstring("spec.vhost: Forbidden: updates on user, userReference, vhost and rabbitmqClusterReference are all forbidden"))

		By("updating permissions successfully")
		Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, permission)).To(Succeed())
		permission.Spec.Permissions.Write = ".*"
		permission.Spec.Permissions.Read = "^$"
		Expect(rmqClusterClient.Update(ctx, permission, &client.UpdateOptions{})).To(Succeed())

		Eventually(func() string {
			var err error
			fetchedPermissionInfo, err = rabbitClient.GetPermissionsIn(permission.Spec.Vhost, username)
			Expect(err).NotTo(HaveOccurred())
			return fetchedPermissionInfo.Write
		}, 20, 2).Should(Equal(".*"))
		Expect(fetchedPermissionInfo).To(MatchFields(IgnoreExtras, Fields{
			"Vhost":     Equal(permission.Spec.Vhost),
			"User":      Equal(username),
			"Configure": Equal(permission.Spec.Permissions.Configure),
			"Read":      Equal("^$"),
			"Write":     Equal(".*"),
		}))

		By("revoking permissions successfully")
		Expect(rmqClusterClient.Delete(ctx, permission)).To(Succeed())
		Eventually(func() int {
			permissionInfos, err := rabbitClient.ListPermissionsOf(username)
			Expect(err).NotTo(HaveOccurred())
			return len(permissionInfos)
		}, 10, 2).Should(Equal(0))
	},

		Entry("grants and revokes permissions successfully when spec.user is set", "User"),
		Entry("grants and revokes permissions successfully when spec.userReference is set", "UserReference"),
	)
})
