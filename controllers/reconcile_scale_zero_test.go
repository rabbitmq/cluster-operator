package controllers_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/v2/internal/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Cluster scale to zero", func() {
	var (
		cluster          *rabbitmqv1beta1.RabbitmqCluster
		defaultNamespace = "default"
		ctx              = context.Background()
	)

	AfterEach(func() {
		Expect(client.Delete(ctx, cluster)).To(Succeed())
		waitForClusterDeletion(ctx, cluster, client)
		Eventually(func() bool {
			rmq := &rabbitmqv1beta1.RabbitmqCluster{}
			err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
			return apierrors.IsNotFound(err)
		}).Should(BeTrue())
	})

	It("scale to zero", func() {
		By("update statefulSet replicas to zero", func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-to-zero",
					Namespace: defaultNamespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: ptr.To(int32(2)),
				},
			}
			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)

			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.Replicas = ptr.To(int32(0))
			})).To(Succeed())

			Eventually(func() int32 {
				sts, err := clientSet.AppsV1().StatefulSets(defaultNamespace).Get(ctx, cluster.ChildResourceName("server"), metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				return *sts.Spec.Replicas
			}, 10, 1).Should(Equal(int32(0)))

		})

		By("setting ReconcileSuccess to 'true'", func() {
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
			}, 0).Should(Equal("ReconcileSuccess status: True, " +
				"with reason: Success " +
				"and message: Finish reconciling"))
		})
	})
})

var _ = Describe("Cluster scale from zero", func() {
	var (
		cluster          *rabbitmqv1beta1.RabbitmqCluster
		defaultNamespace = "default"
		ctx              = context.Background()
	)

	AfterEach(func() {
		Expect(client.Delete(ctx, cluster)).To(Succeed())
		waitForClusterDeletion(ctx, cluster, client)
		Eventually(func() bool {
			rmq := &rabbitmqv1beta1.RabbitmqCluster{}
			err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
			return apierrors.IsNotFound(err)
		}).Should(BeTrue())
	})

	It("scale from zero", func() {
		By("update statefulSet replicas from zero", func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-from-zero",
					Namespace: defaultNamespace,
					Annotations: map[string]string{
						"rabbitmq.com/before-zero-replicas-configured": "2",
					},
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: ptr.To(int32(0)),
				},
			}
			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)

			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.Replicas = ptr.To(int32(2))
			})).To(Succeed())

			Eventually(func() int32 {
				sts, err := clientSet.AppsV1().StatefulSets(defaultNamespace).Get(ctx, cluster.ChildResourceName("server"), metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				return *sts.Spec.Replicas
			}, 10, 1).Should(Equal(int32(2)))

		})

		By("setting ReconcileSuccess to 'true'", func() {
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
			}, 0).Should(Equal("ReconcileSuccess status: True, " +
				"with reason: Success " +
				"and message: Finish reconciling"))
		})
	})
})

var _ = Describe("Cluster scale from zero to less replicas configured", Ordered, func() {
	var (
		cluster          *rabbitmqv1beta1.RabbitmqCluster
		defaultNamespace = "default"
		ctx              = context.Background()
	)

	AfterEach(func() {
		Expect(client.Delete(ctx, cluster)).To(Succeed())
		waitForClusterDeletion(ctx, cluster, client)
		Eventually(func() bool {
			rmq := &rabbitmqv1beta1.RabbitmqCluster{}
			err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
			return apierrors.IsNotFound(err)
		}).Should(BeTrue())
	})

	It("scale from zero to less replicas", func() {
		By("update statefulSet replicas from zero", func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-from-zero-to-less",
					Namespace: defaultNamespace,
					Annotations: map[string]string{
						"rabbitmq.com/before-zero-replicas-configured": "2",
					},
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: ptr.To(int32(0)),
				},
			}
			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)

			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.Replicas = ptr.To(int32(1))
			})).To(Succeed())

			Consistently(func() int32 {
				sts, err := clientSet.AppsV1().StatefulSets(defaultNamespace).Get(ctx, cluster.ChildResourceName("server"), metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				return *sts.Spec.Replicas
			}, 10, 1).Should(Equal(int32(0)))

		})

		By("setting 'Warning' events", func() {
			Expect(aggregateEventMsgs(ctx, cluster, "UnsupportedOperation")).To(
				ContainSubstring("Cluster Scale down not supported"))
		})

		By("setting ReconcileSuccess to 'false'", func() {
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
			}, 0).Should(Equal("ReconcileSuccess status: False, " +
				"with reason: UnsupportedOperation " +
				"and message: Cluster Scale down not supported; tried to scale cluster from 2 nodes to 1 nodes"))
		})
	})
})
