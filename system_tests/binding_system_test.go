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

var _ = Describe("Binding", func() {
	var (
		namespace = MustHaveEnv("NAMESPACE")
		ctx       = context.Background()
		binding   *rabbitmqv1beta1.Binding
		queue     *rabbitmqv1beta1.Queue
		exchange  *rabbitmqv1beta1.Exchange
	)

	BeforeEach(func() {
		exchange = &rabbitmqv1beta1.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-exchange",
				Namespace: namespace,
			},
			Spec: rabbitmqv1beta1.ExchangeSpec{
				Name: "test-exchange",
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: rmq.Name,
				},
			},
		}

		Expect(rmqClusterClient.Create(ctx, exchange, &client.CreateOptions{})).To(Succeed())
		queue = &rabbitmqv1beta1.Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-queue",
				Namespace: namespace,
			},
			Spec: rabbitmqv1beta1.QueueSpec{
				Name: "test-queue",
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: rmq.Name,
				},
			},
		}
		Expect(rmqClusterClient.Create(ctx, queue, &client.CreateOptions{})).To(Succeed())
		Eventually(func() error {
			var err error
			_, err = rabbitClient.GetQueue(queue.Spec.Vhost, queue.Name)
			return err
		}, 10, 2).Should(BeNil()) // wait for queue to be available; or else binding will fail to create

		binding = &rabbitmqv1beta1.Binding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "binding-test",
				Namespace: namespace,
			},
			Spec: rabbitmqv1beta1.BindingSpec{
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: rmq.Name,
				},
				Source:          "test-exchange",
				Destination:     "test-queue",
				DestinationType: "queue",
				RoutingKey:      "test-key",
				Arguments: &runtime.RawExtension{
					Raw: []byte(`{"extra-argument": "test"}`),
				},
			},
		}
	})

	AfterEach(func() {
		Expect(rmqClusterClient.Delete(ctx, queue)).To(Succeed())
		Expect(rmqClusterClient.Delete(ctx, exchange)).To(Succeed())
	})

	It("declares a binding successfully", func() {
		Expect(rmqClusterClient.Create(ctx, binding, &client.CreateOptions{})).To(Succeed())
		var fetchedBinding rabbithole.BindingInfo
		Eventually(func() bool {
			var err error
			bindings, err := rabbitClient.ListBindingsIn(binding.Spec.Vhost)
			Expect(err).NotTo(HaveOccurred())
			for _, b := range bindings {
				if b.Source == binding.Spec.Source {
					fetchedBinding = b
					return true
				}
			}
			return false
		}, 10, 2).Should(BeTrue(), "cannot find created binding")
		Expect(fetchedBinding).To(MatchFields(IgnoreExtras, Fields{
			"Vhost":           Equal(binding.Spec.Vhost),
			"Source":          Equal(binding.Spec.Source),
			"Destination":     Equal(binding.Spec.Destination),
			"DestinationType": Equal(binding.Spec.DestinationType),
			"RoutingKey":      Equal(binding.Spec.RoutingKey),
		}))
		Expect(fetchedBinding.Arguments).To(HaveKeyWithValue("extra-argument", "test"))

		By("updating status condition 'Ready'")
		updatedBinding := rabbitmqv1beta1.Binding{}

		Eventually(func() []rabbitmqv1beta1.Condition {
			Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace}, &updatedBinding)).To(Succeed())
			return updatedBinding.Status.Conditions
		}, waitUpdatedStatusCondition, 2).Should(HaveLen(1), "Binding status condition should be present")

		readyCondition := updatedBinding.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
		Expect(readyCondition.LastTransitionTime).NotTo(Equal(metav1.Time{}))

		By("setting status.observedGeneration")
		Expect(updatedBinding.Status.ObservedGeneration).To(Equal(updatedBinding.GetGeneration()))

		By("not allowing updates on binding.spec")
		updateBinding := rabbitmqv1beta1.Binding{}
		Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace}, &updateBinding)).To(Succeed())
		updatedBinding.Spec.RoutingKey = "new-key"
		Expect(rmqClusterClient.Update(ctx, &updatedBinding).Error()).To(ContainSubstring("invalid: spec.routingKey: Invalid value: \"new-key\": routingKey cannot be updated"))

		By("deleting binding from rabbitmq server")
		Expect(rmqClusterClient.Delete(ctx, binding)).To(Succeed())
		Eventually(func() int {
			var err error
			bindings, err := rabbitClient.ListQueueBindingsBetween(binding.Spec.Vhost, binding.Spec.Source, binding.Spec.Destination)
			Expect(err).NotTo(HaveOccurred())
			return len(bindings)
		}, 10, 2).Should(Equal(0), "cannot find created binding")
	})
})
