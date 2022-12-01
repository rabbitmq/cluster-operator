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

var _ = Describe("federation", func() {
	var (
		namespace           = MustHaveEnv("NAMESPACE")
		ctx                 = context.Background()
		federation          = &rabbitmqv1beta1.Federation{}
		federationUri       = "amqp://server-name-my-upstream-test-uri0,amqp://server-name-my-upstream-test-uri1,amqp://server-name-my-upstream-test-uri2"
		federationUriSecret corev1.Secret
	)

	BeforeEach(func() {
		federationUriSecret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "federation-uri",
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"uri": []byte(federationUri),
			},
		}
		Expect(rmqClusterClient.Create(ctx, &federationUriSecret)).To(Succeed())

		federation = &rabbitmqv1beta1.Federation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "federation",
				Namespace: namespace,
			},
			Spec: rabbitmqv1beta1.FederationSpec{
				Name:       "my-upstream",
				UriSecret:  &corev1.LocalObjectReference{Name: federationUriSecret.Name},
				MessageTTL: 3000,
				Queue:      "a-queue",
				AckMode:    "on-publish",
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: rmq.Name,
				},
			},
		}
	})

	AfterEach(func() {
		Expect(rmqClusterClient.Delete(ctx, &federationUriSecret, &client.DeleteOptions{})).To(Succeed())
	})

	It("works", func() {
		By("federation upstream successfully")
		Expect(rmqClusterClient.Create(ctx, federation, &client.CreateOptions{})).To(Succeed())
		var upstream *rabbithole.FederationUpstream
		Eventually(func() error {
			var err error
			upstream, err = rabbitClient.GetFederationUpstream("/", federation.Spec.Name)
			return err
		}, 30, 2).Should(BeNil())

		Expect(upstream.Name).To(Equal(federation.Spec.Name))
		Expect(upstream.Vhost).To(Equal(federation.Spec.Vhost))
		Expect(upstream.Definition.Uri).To(ConsistOf("amqp://server-name-my-upstream-test-uri0",
			"amqp://server-name-my-upstream-test-uri1",
			"amqp://server-name-my-upstream-test-uri2"))
		Expect(upstream.Definition.Queue).To(Equal(federation.Spec.Queue))
		Expect(upstream.Definition.MessageTTL).To(Equal(int32(federation.Spec.MessageTTL)))
		Expect(upstream.Definition.AckMode).To(Equal(federation.Spec.AckMode))

		By("updating status condition 'Ready'")
		updatedFederation := rabbitmqv1beta1.Federation{}
		Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace}, &updatedFederation)).To(Succeed())

		Eventually(func() []rabbitmqv1beta1.Condition {
			Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace}, &updatedFederation)).To(Succeed())
			return updatedFederation.Status.Conditions
		}, waitUpdatedStatusCondition, 2).Should(HaveLen(1), "Federation status condition should be present")

		readyCondition := updatedFederation.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
		Expect(readyCondition.LastTransitionTime).NotTo(Equal(metav1.Time{}))

		By("setting status.observedGeneration")
		Expect(updatedFederation.Status.ObservedGeneration).To(Equal(updatedFederation.GetGeneration()))

		By("not allowing updates on certain fields")
		updateTest := rabbitmqv1beta1.Federation{}
		Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace}, &updateTest)).To(Succeed())
		updateTest.Spec.Vhost = "/a-new-vhost"
		Expect(rmqClusterClient.Update(ctx, &updateTest).Error()).To(ContainSubstring("spec.vhost: Forbidden: updates on name, vhost and rabbitmqClusterReference are all forbidden"))

		By("updating federation upstream parameters successfully")
		Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace}, federation)).To(Succeed())
		federation.Spec.MessageTTL = 1000
		Expect(rmqClusterClient.Update(ctx, federation, &client.UpdateOptions{})).To(Succeed())

		Eventually(func() int32 {
			var err error
			upstream, err = rabbitClient.GetFederationUpstream("/", federation.Spec.Name)
			Expect(err).NotTo(HaveOccurred())
			return upstream.Definition.MessageTTL
		}, 30, 2).Should(Equal(int32(1000)))

		By("unsetting federation upstream on deletion")
		Expect(rmqClusterClient.Delete(ctx, federation)).To(Succeed())
		var err error
		Eventually(func() error {
			_, err = rabbitClient.GetFederationUpstream("/", federation.Spec.Name)
			return err
		}, 10).Should(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Object Not Found"))
	})
})
