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
)

var _ = Describe("Shovel", func() {
	var (
		namespace    = MustHaveEnv("NAMESPACE")
		ctx          = context.Background()
		shovel       = &rabbitmqv1beta1.Shovel{}
		srcUri       = "amqp://server-test-src0,amqp://server-test-src1"
		destUri      = "amqp://server-test-dest0,amqp://server-test-dest1"
		shovelSecret corev1.Secret
	)

	BeforeEach(func() {
		shovelSecret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shovel-uri",
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"srcUri":  []byte(srcUri),
				"destUri": []byte(destUri),
			},
		}
		Expect(rmqClusterClient.Create(ctx, &shovelSecret)).To(Succeed())

		shovel = &rabbitmqv1beta1.Shovel{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shovel",
				Namespace: namespace,
			},
			Spec: rabbitmqv1beta1.ShovelSpec{
				Name:             "my-upstream",
				UriSecret:        &corev1.LocalObjectReference{Name: shovelSecret.Name},
				DeleteAfter:      "never",
				SourceQueue:      "a-queue",
				DestinationQueue: "another-queue",
				AckMode:          "no-ack",
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: rmq.Name,
				},
			},
		}
	})

	AfterEach(func() {
		Expect(rmqClusterClient.Delete(ctx, &shovelSecret, &client.DeleteOptions{})).To(Succeed())
	})

	It("works", func() {
		By("declaring shovel successfully")
		Expect(rmqClusterClient.Create(ctx, shovel, &client.CreateOptions{})).To(Succeed())
		var shovelInfo *rabbithole.ShovelInfo
		Eventually(func() error {
			var err error
			shovelInfo, err = rabbitClient.GetShovel("/", shovel.Spec.Name)
			return err
		}, 30, 2).Should(BeNil())

		Expect(shovelInfo.Name).To(Equal(shovel.Spec.Name))
		Expect(shovelInfo.Vhost).To(Equal(shovel.Spec.Vhost))
		Expect(shovelInfo.Definition.SourceURI).To(
			ConsistOf("amqp://server-test-src0",
				"amqp://server-test-src1"))
		Expect(shovelInfo.Definition.DestinationURI).To(
			ConsistOf("amqp://server-test-dest0",
				"amqp://server-test-dest1"))
		Expect(shovelInfo.Definition.DestinationQueue).To(Equal(shovel.Spec.DestinationQueue))
		Expect(shovelInfo.Definition.SourceQueue).To(Equal(shovel.Spec.SourceQueue))
		Expect(shovelInfo.Definition.AckMode).To(Equal(shovel.Spec.AckMode))
		Expect(string(shovelInfo.Definition.DeleteAfter)).To(Equal(shovel.Spec.DeleteAfter))

		By("updating status condition 'Ready'")
		updatedShovel := rabbitmqv1beta1.Shovel{}

		Eventually(func() []rabbitmqv1beta1.Condition {
			Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace}, &updatedShovel)).To(Succeed())
			return updatedShovel.Status.Conditions
		}, waitUpdatedStatusCondition, 2).Should(HaveLen(1), "Shovel status condition should be present")

		readyCondition := updatedShovel.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
		Expect(readyCondition.LastTransitionTime).NotTo(Equal(metav1.Time{}))

		By("setting status.observedGeneration")
		Expect(updatedShovel.Status.ObservedGeneration).To(Equal(updatedShovel.GetGeneration()))

		By("not allowing updates on certain fields")
		updateTest := rabbitmqv1beta1.Shovel{}
		Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace}, &updateTest)).To(Succeed())
		updateTest.Spec.Name = "a-new-shovel"
		Expect(rmqClusterClient.Update(ctx, &updateTest).Error()).To(ContainSubstring("spec.name: Forbidden: updates on name, vhost and rabbitmqClusterReference are all forbidden"))

		By("updating shovel upstream parameters successfully")
		Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace}, shovel)).To(Succeed())
		shovel.Spec.PrefetchCount = 200
		Expect(rmqClusterClient.Update(ctx, shovel, &client.UpdateOptions{})).To(Succeed())

		Eventually(func() int {
			info, err := rabbitClient.GetShovel("/", shovel.Spec.Name)
			Expect(err).NotTo(HaveOccurred())
			return info.Definition.PrefetchCount
		}, 30, 2).Should(Equal(200))

		By("deleting shovel configuration on deletion")
		Expect(rmqClusterClient.Delete(ctx, shovel)).To(Succeed())
		var err error
		Eventually(func() error {
			_, err = rabbitClient.GetShovel("/", shovel.Spec.Name)
			return err
		}, 10).Should(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Object Not Found"))
	})
})
