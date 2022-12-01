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

var _ = Describe("vhost", func() {

	var (
		namespace = MustHaveEnv("NAMESPACE")
		ctx       = context.Background()
		vhost     = &rabbitmqv1beta1.Vhost{}
	)

	BeforeEach(func() {
		vhost = &rabbitmqv1beta1.Vhost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: namespace,
			},
			Spec: rabbitmqv1beta1.VhostSpec{
				Name: "test",
				Tags: []string{"multi_dc_replication"},
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: rmq.Name,
				},
			},
		}
	})

	It("creates and deletes a vhost successfully", func() {
		By("creating a vhost")
		Expect(rmqClusterClient.Create(ctx, vhost, &client.CreateOptions{})).To(Succeed())
		var fetched *rabbithole.VhostInfo
		Eventually(func() error {
			var err error
			fetched, err = rabbitClient.GetVhost(vhost.Spec.Name)
			return err
		}, 30, 2).ShouldNot(HaveOccurred(), "cannot find created vhost")
		Expect(fetched.Tracing).To(BeFalse())
		Expect(fetched.Tags).To(HaveLen(1))
		Expect(fetched.Tags[0]).To(Equal("multi_dc_replication"))

		By("updating status condition 'Ready'")
		updatedVhost := rabbitmqv1beta1.Vhost{}

		Eventually(func() []rabbitmqv1beta1.Condition {
			Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace}, &updatedVhost)).To(Succeed())
			return updatedVhost.Status.Conditions
		}, waitUpdatedStatusCondition, 2).Should(HaveLen(1), "Vhost status condition should be present")

		readyCondition := updatedVhost.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
		Expect(readyCondition.LastTransitionTime).NotTo(Equal(metav1.Time{}))

		By("setting status.observedGeneration")
		Expect(updatedVhost.Status.ObservedGeneration).To(Equal(updatedVhost.GetGeneration()))

		By("not allowing updates on certain fields")
		updateTest := rabbitmqv1beta1.Vhost{}
		Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace}, &updateTest)).To(Succeed())
		updateTest.Spec.Name = "new-name"
		Expect(rmqClusterClient.Update(ctx, &updateTest).Error()).To(ContainSubstring("spec.name: Forbidden: updates on name and rabbitmqClusterReference are all forbidden"))

		By("deleting a vhost")
		Expect(rmqClusterClient.Delete(ctx, vhost)).To(Succeed())
		var err error
		Eventually(func() error {
			_, err = rabbitClient.GetVhost(vhost.Spec.Name)
			return err
		}, 30).Should(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Object Not Found"))
	})
})
