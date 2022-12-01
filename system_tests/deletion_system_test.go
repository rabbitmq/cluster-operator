package system_tests

import (
	"context"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Deletion", func() {
	var (
		namespace     = MustHaveEnv("NAMESPACE")
		ctx           = context.Background()
		targetCluster *rabbitmqv1beta1.RabbitmqCluster
		exchange      rabbitmqv1beta1.Exchange
		policy        rabbitmqv1beta1.Policy
		queue         rabbitmqv1beta1.Queue
		user          rabbitmqv1beta1.User
		vhost         rabbitmqv1beta1.Vhost
	)

	BeforeEach(func() {
		targetCluster = basicTestRabbitmqCluster("to-be-deleted", namespace)
		targetCluster.Spec.TerminationGracePeriodSeconds = pointer.Int64Ptr(10)
		setupTestRabbitmqCluster(rmqClusterClient, targetCluster)
		targetClusterRef := rabbitmqv1beta1.RabbitmqClusterReference{Name: targetCluster.Name}
		exchange = rabbitmqv1beta1.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "exchange-deletion-test",
				Namespace: namespace,
			},
			Spec: rabbitmqv1beta1.ExchangeSpec{
				Name:                     "exchange-deletion-test",
				RabbitmqClusterReference: targetClusterRef,
			},
		}
		policy = rabbitmqv1beta1.Policy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "policy-deletion-test",
				Namespace: namespace,
			},
			Spec: rabbitmqv1beta1.PolicySpec{
				Name:    "policy-deletion-test",
				Pattern: ".*",
				ApplyTo: "queues",
				Definition: &runtime.RawExtension{
					Raw: []byte(`{"ha-mode":"all"}`),
				},
				RabbitmqClusterReference: targetClusterRef,
			},
		}
		queue = rabbitmqv1beta1.Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "queue-deletion-test",
				Namespace: namespace,
			},
			Spec: rabbitmqv1beta1.QueueSpec{
				Name:                     "queue-deletion-test",
				RabbitmqClusterReference: targetClusterRef,
			},
		}
		user = rabbitmqv1beta1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "user-deletion-test",
				Namespace: namespace,
			},
			Spec: rabbitmqv1beta1.UserSpec{
				RabbitmqClusterReference: targetClusterRef,
			},
		}
		vhost = rabbitmqv1beta1.Vhost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vhost-deletion-test",
				Namespace: namespace,
			},
			Spec: rabbitmqv1beta1.VhostSpec{
				Name:                     "vhost-deletion-test",
				RabbitmqClusterReference: targetClusterRef,
			},
		}
		Expect(rmqClusterClient.Create(ctx, &exchange)).To(Succeed())
		Expect(rmqClusterClient.Create(ctx, &policy)).To(Succeed())
		Expect(rmqClusterClient.Create(ctx, &queue)).To(Succeed())
		Expect(rmqClusterClient.Create(ctx, &user)).To(Succeed())
		Expect(rmqClusterClient.Create(ctx, &vhost)).To(Succeed())
	})

	It("handles the referenced RabbitmqCluster being deleted", func() {
		Expect(rmqClusterClient.Delete(ctx, &rabbitmqv1beta1.RabbitmqCluster{ObjectMeta: metav1.ObjectMeta{Name: targetCluster.Name, Namespace: targetCluster.Namespace}})).To(Succeed())
		Eventually(func() string {
			output, _ := kubectl(
				"-n",
				targetCluster.Namespace,
				"get",
				"rabbitmqclusters",
				targetCluster.Name,
			)
			return string(output)
		}, 90, 10).Should(ContainSubstring("NotFound"))
		By("allowing the topology objects to be deleted")
		Expect(rmqClusterClient.Delete(ctx, &exchange)).To(Succeed())
		Eventually(func() string {
			output, _ := kubectl(
				"-n",
				exchange.Namespace,
				"get",
				"exchange",
				exchange.Name,
			)
			return string(output)
		}, 30, 10).Should(ContainSubstring("NotFound"))
		Expect(rmqClusterClient.Delete(ctx, &policy)).To(Succeed())
		Eventually(func() string {
			output, _ := kubectl(
				"-n",
				policy.Namespace,
				"get",
				"policy",
				policy.Name,
			)
			return string(output)
		}, 30, 10).Should(ContainSubstring("NotFound"))
		Expect(rmqClusterClient.Delete(ctx, &queue)).To(Succeed())
		Eventually(func() string {
			output, _ := kubectl(
				"-n",
				queue.Namespace,
				"get",
				"queue",
				queue.Name,
			)
			return string(output)
		}, 30, 10).Should(ContainSubstring("NotFound"))
		Expect(rmqClusterClient.Delete(ctx, &user)).To(Succeed())
		Eventually(func() string {
			output, _ := kubectl(
				"-n",
				user.Namespace,
				"get",
				"user",
				user.Name,
			)
			return string(output)
		}, 30, 10).Should(ContainSubstring("NotFound"))
		Expect(rmqClusterClient.Delete(ctx, &vhost)).To(Succeed())
		Eventually(func() string {
			output, _ := kubectl(
				"-n",
				vhost.Namespace,
				"get",
				"vhost",
				vhost.Name,
			)
			return string(output)
		}, 30, 10).Should(ContainSubstring("NotFound"))
	})
})
