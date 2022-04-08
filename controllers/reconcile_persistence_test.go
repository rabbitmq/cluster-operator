package controllers_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/status"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Persistence", func() {
	var (
		cluster          *rabbitmqv1beta1.RabbitmqCluster
		defaultNamespace = "default"
		ctx              = context.Background()
	)

	BeforeEach(func() {
		cluster = &rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rabbitmq-shrink",
				Namespace: defaultNamespace,
			},
			Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
				Replicas: pointer.Int32Ptr(5),
			},
		}
		Expect(client.Create(ctx, cluster)).To(Succeed())
		waitForClusterCreation(ctx, cluster, client)
	})

	AfterEach(func() {
		Expect(client.Delete(ctx, cluster)).To(Succeed())
		Eventually(func() bool {
			rmq := &rabbitmqv1beta1.RabbitmqCluster{}
			err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
			return apierrors.IsNotFound(err)
		}, 5).Should(BeTrue())
	})

	It("does not allow PVC shrink", func() {
		By("not updating statefulSet volume claim storage capacity", func() {
			tenG := k8sresource.MustParse("10Gi")
			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				storage := k8sresource.MustParse("1Gi")
				cluster.Spec.Persistence.Storage = &storage
			})).To(Succeed())
			Consistently(func() k8sresource.Quantity {
				sts, err := clientSet.AppsV1().StatefulSets(defaultNamespace).Get(ctx, cluster.ChildResourceName("server"), metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				return sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]
			}, 10, 1).Should(Equal(tenG))
		})

		By("setting 'Warning' events", func() {
			Expect(aggregateEventMsgs(ctx, cluster, "FailedReconcilePersistence")).To(
				ContainSubstring("shrinking persistent volumes is not supported"))
		})

		By("setting ReconcileSuccess to 'false' with failed reason and message", func() {
			Eventually(func() string {
				rabbit := &rabbitmqv1beta1.RabbitmqCluster{}
				Expect(client.Get(ctx, runtimeClient.ObjectKey{
					Name:      cluster.Name,
					Namespace: defaultNamespace,
				}, rabbit)).To(Succeed())

				for i := range rabbit.Status.Conditions {
					if rabbit.Status.Conditions[i].Type == status.ReconcileSuccess {
						return fmt.Sprintf(
							"ReconcileSuccess status: %s, with reason: %s and message: %s",
							rabbit.Status.Conditions[i].Status,
							rabbit.Status.Conditions[i].Reason,
							rabbit.Status.Conditions[i].Message)
					}
				}
				return "ReconcileSuccess status: condition not present"
			}, 5).Should(Equal("ReconcileSuccess status: False, " +
				"with reason: FailedReconcilePVC " +
				"and message: shrinking persistent volumes is not supported"))
		})
	})
})
