package controllers_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/v2/internal/status"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Cluster scale to zero", func() {
	var (
		cluster          *rabbitmqv1beta1.RabbitmqCluster
		defaultNamespace = "default"
		ctx              = context.Background()
	)

	It("scale to zero", func() {
		By("update statefulSet replicas to zero", func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				Name:      "rabbitmq-to-zero",
				Namespace: defaultNamespace,
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: new(int32(2)),
				},
			}
			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)

			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.Replicas = new(int32(0))
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

	It("scale from zero", func() {
		By("update statefulSet replicas from zero", func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				Name:      "rabbitmq-from-zero",
				Namespace: defaultNamespace,
				Annotations: map[string]string{
					"rabbitmq.com/before-zero-replicas-configured": "2",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: new(int32(0)),
				},
			}
			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)

			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.Replicas = new(int32(2))
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

	It("scale from zero to less replicas", func() {
		By("update statefulSet replicas from zero", func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				Name:      "rabbitmq-from-zero-to-less",
				Namespace: defaultNamespace,
				Annotations: map[string]string{
					"rabbitmq.com/before-zero-replicas-configured": "2",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: new(int32(0)),
				},
			}
			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)

			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.Replicas = new(int32(1))
			})).To(Succeed())

			Consistently(func() int32 {
				sts, err := clientSet.AppsV1().StatefulSets(defaultNamespace).Get(ctx, cluster.ChildResourceName("server"), metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				return *sts.Spec.Replicas
			}, 10, 1).Should(Equal(int32(0)))

		})

		By("setting 'Warning' events", func() {
			Expect(aggregateEventMsgs(ctx, cluster, "UnsupportedOperation")).To(
				ContainSubstring("Unsupported operation"))
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
				"and message: Unsupported operation; when scaling from zero, you can only restore the previous number of replicas (2)"))
		})
	})
})

var _ = Describe("Cluster scale through the scale subresource", func() {
	var (
		cluster          *rabbitmqv1beta1.RabbitmqCluster
		defaultNamespace = "default"
		ctx              = context.Background()
	)

	scaleTo := func(replicas int32) {
		GinkgoHelper()
		Eventually(func() error {
			current := &rabbitmqv1beta1.RabbitmqCluster{}
			if err := client.Get(ctx, runtimeClient.ObjectKeyFromObject(cluster), current); err != nil {
				return err
			}
			return client.SubResource("scale").Update(ctx, current,
				runtimeClient.WithSubResourceBody(&autoscalingv1.Scale{
					ObjectMeta: metav1.ObjectMeta{
						Name:            current.Name,
						Namespace:       current.Namespace,
						ResourceVersion: current.ResourceVersion,
					},
					Spec: autoscalingv1.ScaleSpec{Replicas: replicas},
				}))
		}, 10, 1).Should(Succeed())
	}

	stsReplicas := func() int32 {
		sts, err := clientSet.AppsV1().StatefulSets(defaultNamespace).Get(ctx, cluster.ChildResourceName("server"), metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		return *sts.Spec.Replicas
	}

	It("scales to zero and back, and reports status.replicas", func() {
		cluster = &rabbitmqv1beta1.RabbitmqCluster{
			Name:      "rabbitmq-scale-subresource",
			Namespace: defaultNamespace,
			Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
				Replicas: new(int32(2)),
			},
		}
		Expect(client.Create(ctx, cluster)).To(Succeed())
		waitForClusterCreation(ctx, cluster, client)

		By("scaling to zero through /scale", func() {
			scaleTo(0)

			Eventually(func() int32 {
				current := &rabbitmqv1beta1.RabbitmqCluster{}
				Expect(client.Get(ctx, runtimeClient.ObjectKeyFromObject(cluster), current)).To(Succeed())
				return *current.Spec.Replicas
			}, 10, 1).Should(Equal(int32(0)))
			Eventually(stsReplicas, 10, 1).Should(Equal(int32(0)))

			Eventually(func() string {
				current := &rabbitmqv1beta1.RabbitmqCluster{}
				Expect(client.Get(ctx, runtimeClient.ObjectKeyFromObject(cluster), current)).To(Succeed())
				return current.Annotations["rabbitmq.com/before-zero-replicas-configured"]
			}, 10, 1).Should(Equal("2"))
		})

		By("reporting status.replicas from the StatefulSet status", func() {
			// envtest has no StatefulSet controller; set the status manually
			stsName := cluster.ChildResourceName("server")
			Eventually(func() error {
				sts, err := clientSet.AppsV1().StatefulSets(defaultNamespace).Get(ctx, stsName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				sts.Status.Replicas = 2
				_, err = clientSet.AppsV1().StatefulSets(defaultNamespace).UpdateStatus(ctx, sts, metav1.UpdateOptions{})
				return err
			}, 10, 1).Should(Succeed())

			Eventually(func() int32 {
				current := &rabbitmqv1beta1.RabbitmqCluster{}
				Expect(client.Get(ctx, runtimeClient.ObjectKeyFromObject(cluster), current)).To(Succeed())
				return current.Status.Replicas
			}, 15, 1).Should(Equal(int32(2)))

			Eventually(func() error {
				sts, err := clientSet.AppsV1().StatefulSets(defaultNamespace).Get(ctx, stsName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				sts.Status.Replicas = 0
				_, err = clientSet.AppsV1().StatefulSets(defaultNamespace).UpdateStatus(ctx, sts, metav1.UpdateOptions{})
				return err
			}, 10, 1).Should(Succeed())

			Eventually(func() int32 {
				current := &rabbitmqv1beta1.RabbitmqCluster{}
				Expect(client.Get(ctx, runtimeClient.ObjectKeyFromObject(cluster), current)).To(Succeed())
				return current.Status.Replicas
			}, 15, 1).Should(Equal(int32(0)))
		})

		By("scaling back to the previous count through /scale", func() {
			scaleTo(2)
			Eventually(stsReplicas, 10, 1).Should(Equal(int32(2)))
		})
	})
})
