package controllers_test

import (
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Reconcile finalizer", func() {
	var (
		cluster          *rabbitmqv1beta1.RabbitmqCluster
		defaultNamespace = "default"
	)

	BeforeEach(func() {
		cluster = &rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rabbitmq-finalizer",
				Namespace: defaultNamespace,
			},
		}

		Expect(client.Create(ctx, cluster)).To(Succeed())
		waitForClusterCreation(ctx, cluster, client)
	})

	It("adds the deletion finalizer", func() {
		rmq := &rabbitmqv1beta1.RabbitmqCluster{}
		Eventually(func() string {
			err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
			if err != nil {
				return ""
			}
			if len(rmq.Finalizers) > 0 {
				return rmq.Finalizers[0]
			}

			return ""
		}, 5).Should(Equal("deletion.finalizers.rabbitmqclusters.rabbitmq.com"))
	})
})
