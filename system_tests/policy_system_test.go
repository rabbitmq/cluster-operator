package system_tests

import (
	"context"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Policy", func() {
	var (
		namespace = MustHaveEnv("NAMESPACE")
		ctx       = context.Background()
		policy    *rabbitmqv1beta1.Policy
	)

	BeforeEach(func() {
		policy = &rabbitmqv1beta1.Policy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "policy-test",
				Namespace: namespace,
			},
			Spec: rabbitmqv1beta1.PolicySpec{
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: rmq.Name,
				},
				Name:    "policy-test",
				Pattern: "test-queue",
				ApplyTo: "queues",
				Definition: &runtime.RawExtension{
					Raw: []byte(`{"ha-mode":"all"}`),
				},
			},
		}
	})

	It("creates, updates and deletes a policy successfully", func() {
		By("creating policy")
		Expect(rmqClusterClient.Create(ctx, policy, &client.CreateOptions{})).To(Succeed())
		var fetchedPolicy *rabbithole.Policy
		Eventually(func() error {
			var err error
			fetchedPolicy, err = rabbitClient.GetPolicy(policy.Spec.Vhost, policy.Name)
			return err
		}, 10, 2).Should(BeNil())

		Expect(*fetchedPolicy).To(MatchFields(IgnoreExtras, Fields{
			"Name":     Equal(policy.Spec.Name),
			"Vhost":    Equal(policy.Spec.Vhost),
			"Pattern":  Equal("test-queue"),
			"ApplyTo":  Equal("queues"),
			"Priority": Equal(0),
		}))

		Expect(fetchedPolicy.Definition).To(HaveKeyWithValue("ha-mode", "all"))

		By("updating status condition 'Ready'")
		updatedPolicy := rabbitmqv1beta1.Policy{}

		Eventually(func() []rabbitmqv1beta1.Condition {
			Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace}, &updatedPolicy)).To(Succeed())
			return updatedPolicy.Status.Conditions
		}, waitUpdatedStatusCondition, 2).Should(HaveLen(1), "Policy status condition should be present")

		readyCondition := updatedPolicy.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
		Expect(readyCondition.LastTransitionTime).NotTo(Equal(metav1.Time{}))

		By("setting status.observedGeneration")
		Expect(updatedPolicy.Status.ObservedGeneration).To(Equal(updatedPolicy.GetGeneration()))

		By("not allowing updates on certain fields")
		updateTest := rabbitmqv1beta1.Policy{}
		Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace}, &updateTest)).To(Succeed())
		updateTest.Spec.Vhost = "/a-new-vhost"
		Expect(rmqClusterClient.Update(ctx, &updateTest).Error()).To(ContainSubstring("spec.vhost: Forbidden: updates on name, vhost and rabbitmqClusterReference are all forbidden"))

		By("updating policy definitions successfully")
		Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace}, policy)).To(Succeed())
		policy.Spec.Definition = &runtime.RawExtension{
			Raw: []byte(`{"ha-mode":"exactly",
"ha-params": 2
}`)}
		Expect(rmqClusterClient.Update(ctx, policy, &client.UpdateOptions{})).To(Succeed())

		Eventually(func() rabbithole.PolicyDefinition {
			var err error
			fetchedPolicy, err = rabbitClient.GetPolicy(policy.Spec.Vhost, policy.Name)
			Expect(err).NotTo(HaveOccurred())
			return fetchedPolicy.Definition
		}, 10, 2).Should(HaveLen(2))

		Expect(fetchedPolicy.Definition).To(HaveKeyWithValue("ha-mode", "exactly"))
		Expect(fetchedPolicy.Definition).To(HaveKeyWithValue("ha-params", float64(2)))

		By("deleting policy")
		Expect(rmqClusterClient.Delete(ctx, policy)).To(Succeed())
		var err error
		Eventually(func() error {
			_, err = rabbitClient.GetPolicy(policy.Spec.Vhost, policy.Name)
			return err
		}, 10).Should(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Object Not Found"))
	})
})
