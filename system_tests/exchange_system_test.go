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

var _ = Describe("Exchange", func() {
	var (
		namespace = MustHaveEnv("NAMESPACE")
		ctx       = context.Background()
		exchange  *rabbitmqv1beta1.Exchange
	)

	BeforeEach(func() {
		exchange = &rabbitmqv1beta1.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "exchange-test",
				Namespace: namespace,
			},
			Spec: rabbitmqv1beta1.ExchangeSpec{
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: rmq.Name,
				},
				Name:       "exchange-test",
				Type:       "fanout",
				AutoDelete: false,
				Durable:    true,
				Arguments: &runtime.RawExtension{
					Raw: []byte(`{"alternate-exchange": "system-test"}`),
				},
			},
		}
	})

	It("declares and deletes a exchange successfully", func() {
		By("declaring exchange")
		Expect(rmqClusterClient.Create(ctx, exchange, &client.CreateOptions{})).To(Succeed())
		var exchangeInfo *rabbithole.DetailedExchangeInfo
		Eventually(func() error {
			var err error
			exchangeInfo, err = rabbitClient.GetExchange(exchange.Spec.Vhost, exchange.Name)
			return err
		}, waitUpdatedStatusCondition, 2).Should(BeNil())

		Expect(*exchangeInfo).To(MatchFields(IgnoreExtras, Fields{
			"Name":       Equal(exchange.Spec.Name),
			"Vhost":      Equal(exchange.Spec.Vhost),
			"Type":       Equal(exchange.Spec.Type),
			"AutoDelete": BeFalse(),
			"Durable":    BeTrue(),
		}))
		Expect(exchangeInfo.Arguments).To(HaveKeyWithValue("alternate-exchange", "system-test"))

		By("updating status condition 'Ready'")
		fetched := rabbitmqv1beta1.Exchange{}

		Eventually(func() []rabbitmqv1beta1.Condition {
			Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace}, &fetched)).To(Succeed())
			return fetched.Status.Conditions
		}, 20, 2).Should(HaveLen(1), "Exchange status condition should be present")

		readyCondition := fetched.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
		Expect(readyCondition.LastTransitionTime).NotTo(Equal(metav1.Time{}))

		By("setting status.observedGeneration")
		Expect(fetched.Status.ObservedGeneration).To(Equal(fetched.GetGeneration()))

		By("not allowing certain updates")
		updatedExchange := rabbitmqv1beta1.Exchange{}
		Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace}, &updatedExchange)).To(Succeed())
		updatedExchange.Spec.Vhost = "/new-vhost"
		Expect(rmqClusterClient.Update(ctx, &updatedExchange).Error()).To(ContainSubstring("spec.vhost: Forbidden: updates on name, vhost, and rabbitmqClusterReference are all forbidden"))
		updatedExchange.Spec.Vhost = exchange.Spec.Vhost
		updatedExchange.Spec.Durable = false
		Expect(rmqClusterClient.Update(ctx, &updatedExchange).Error()).To(ContainSubstring("spec.durable: Invalid value: false: durable cannot be updated"))

		By("deleting exchange")
		Expect(rmqClusterClient.Delete(ctx, exchange)).To(Succeed())
		var err error
		Eventually(func() error {
			_, err = rabbitClient.GetExchange(exchange.Spec.Vhost, exchange.Name)
			return err
		}, 30).Should(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Object Not Found"))
	})
})
