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

var _ = Describe("Topic Permission", func() {
	var (
		namespace       = MustHaveEnv("NAMESPACE")
		ctx             = context.Background()
		topicPermission *rabbitmqv1beta1.TopicPermission
		user            *rabbitmqv1beta1.User
		exchange        *rabbitmqv1beta1.Exchange
		username        string
	)

	BeforeEach(func() {
		user = &rabbitmqv1beta1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "userabc",
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
			Name:      "userabc-user-credentials",
			Namespace: namespace,
		}
		var generatedSecret = &corev1.Secret{}
		Eventually(func() error {
			return rmqClusterClient.Get(ctx, generatedSecretKey, generatedSecret)
		}, 30, 2).Should(Succeed())
		username = string(generatedSecret.Data["username"])

		exchange = &rabbitmqv1beta1.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "exchangeabc",
				Namespace: namespace,
			},
			Spec: rabbitmqv1beta1.ExchangeSpec{
				Name: "exchangeabc",
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: rmq.Name,
				},
			},
		}
		Expect(rmqClusterClient.Create(ctx, exchange, &client.CreateOptions{})).To(Succeed())

		topicPermission = &rabbitmqv1beta1.TopicPermission{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-topic-perm",
				Namespace: namespace,
			},
			Spec: rabbitmqv1beta1.TopicPermissionSpec{
				Vhost: "/",
				User:  username,
				Permissions: rabbitmqv1beta1.TopicPermissionConfig{
					Exchange: exchange.Spec.Name,
					Read:     ".*",
					Write:    ".*",
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
		Expect(rmqClusterClient.Delete(ctx, exchange)).To(Succeed())
		Eventually(func() string {
			if err := rmqClusterClient.Get(ctx, types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace}, &rabbitmqv1beta1.Exchange{}); err != nil {
				return err.Error()
			}
			return ""
		}, 10).Should(ContainSubstring("not found"))
	})

	DescribeTable("Server configurations updates", func(testcase string) {
		if testcase == "UserReference" {
			topicPermission.Spec.User = ""
			topicPermission.Spec.UserReference = &corev1.LocalObjectReference{Name: user.Name}
		}
		Expect(rmqClusterClient.Create(ctx, topicPermission, &client.CreateOptions{})).To(Succeed())
		var fetchedPermissionInfo []rabbithole.TopicPermissionInfo
		Eventually(func() error {
			var err error
			fetchedPermissionInfo, err = rabbitClient.GetTopicPermissionsIn(topicPermission.Spec.Vhost, username)
			return err
		}, 30, 2).Should(Not(HaveOccurred()))
		Expect(fetchedPermissionInfo).To(HaveLen(1))
		Expect(fetchedPermissionInfo[0]).To(
			MatchFields(IgnoreExtras, Fields{
				"Vhost":    Equal(topicPermission.Spec.Vhost),
				"User":     Equal(username),
				"Exchange": Equal(topicPermission.Spec.Permissions.Exchange),
				"Read":     Equal(topicPermission.Spec.Permissions.Read),
				"Write":    Equal(topicPermission.Spec.Permissions.Write)}))

		By("updating status condition 'Ready'")
		updated := rabbitmqv1beta1.TopicPermission{}

		Eventually(func() []rabbitmqv1beta1.Condition {
			Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: topicPermission.Name, Namespace: topicPermission.Namespace}, &updated)).To(Succeed())
			return updated.Status.Conditions
		}, waitUpdatedStatusCondition, 2).Should(HaveLen(1), "status condition should be present")

		readyCondition := updated.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
		Expect(readyCondition.LastTransitionTime).NotTo(Equal(metav1.Time{}))

		By("setting status.observedGeneration")
		Expect(updated.Status.ObservedGeneration).To(Equal(updated.GetGeneration()))

		By("not allowing updates on certain fields")
		updateTest := rabbitmqv1beta1.TopicPermission{}
		Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: topicPermission.Name, Namespace: topicPermission.Namespace}, &updateTest)).To(Succeed())
		updateTest.Spec.Vhost = "/a-new-vhost"
		Expect(rmqClusterClient.Update(ctx, &updateTest).Error()).To(ContainSubstring("spec.vhost: Forbidden: updates on exchange, user, userReference, vhost and rabbitmqClusterReference are all forbidden"))

		By("updating topic permissions successfully")
		Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: topicPermission.Name, Namespace: topicPermission.Namespace}, topicPermission)).To(Succeed())
		topicPermission.Spec.Permissions.Write = "^$"
		topicPermission.Spec.Permissions.Read = "^$"
		Expect(rmqClusterClient.Update(ctx, topicPermission, &client.UpdateOptions{})).To(Succeed())

		Eventually(func() string {
			var err error
			fetchedPermissionInfo, err = rabbitClient.GetTopicPermissionsIn(topicPermission.Spec.Vhost, username)
			Expect(err).NotTo(HaveOccurred())
			return fetchedPermissionInfo[0].Write
		}, 20, 2).Should(Equal("^$"))
		Expect(fetchedPermissionInfo).To(HaveLen(1))
		Expect(fetchedPermissionInfo[0]).To(
			MatchFields(IgnoreExtras, Fields{
				"Vhost":    Equal(topicPermission.Spec.Vhost),
				"User":     Equal(username),
				"Exchange": Equal(topicPermission.Spec.Permissions.Exchange),
				"Read":     Equal("^$"),
				"Write":    Equal("^$")}))

		By("clearing permissions successfully")
		Expect(rmqClusterClient.Delete(ctx, topicPermission)).To(Succeed())
		Eventually(func() int {
			permList, err := rabbitClient.ListTopicPermissionsOf(username)
			Expect(err).NotTo(HaveOccurred())
			return len(permList)
		}, 10, 2).Should(Equal(0))
	},

		Entry("manage topic permissions successfully when spec.user is set", "User"),
		Entry("manage topic permissions successfully when spec.userReference is set", "UserReference"),
	)
})
