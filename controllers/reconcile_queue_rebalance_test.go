package controllers_test

import (
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
)

var _ = Describe("Reconcile queue Rebalance", func() {
	var (
		cluster          *rabbitmqv1beta1.RabbitmqCluster
		annotations      map[string]string
		defaultNamespace = "default"
	)

	BeforeEach(func() {
		annotations = map[string]string{}
	})

	AfterEach(func() {
		Expect(client.Delete(ctx, cluster)).To(Succeed())
		waitForClusterDeletion(ctx, cluster, client)
	})

	When("the cluster is configured to run post-deploy steps", func() {
		BeforeEach(func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-three",
					Namespace: defaultNamespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: pointer.Int32Ptr(3),
				},
			}
			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)
		})
		When("the cluster is updated", func() {
			var sts *appsv1.StatefulSet

			BeforeEach(func() {
				sts = statefulSet(ctx, cluster)
				sts.Status.Replicas = 3
				sts.Status.CurrentReplicas = 2
				sts.Status.CurrentRevision = "some-old-revision"
				sts.Status.UpdatedReplicas = 1
				sts.Status.UpdateRevision = "some-new-revision"

				statusWriter := client.Status()
				err := statusWriter.Update(ctx, sts)
				Expect(err).NotTo(HaveOccurred())
			})

			It("triggers the controller to run rabbitmq-queues rebalance all", func() {
				By("setting an annotation on the CR", func() {
					Eventually(func() map[string]string {
						rmq := &rabbitmqv1beta1.RabbitmqCluster{}
						err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
						Expect(err).To(BeNil())
						annotations = rmq.ObjectMeta.Annotations
						return annotations
					}, 5).Should(HaveKey("rabbitmq.com/queueRebalanceNeededAt"))
					_, err := time.Parse(time.RFC3339, annotations["rabbitmq.com/queueRebalanceNeededAt"])
					Expect(err).NotTo(HaveOccurred(), "Annotation rabbitmq.com/queueRebalanceNeededAt was not a valid RFC3339 timestamp")
				})

				By("not removing the annotation when all replicas are updated but not yet ready", func() {
					sts.Status.CurrentReplicas = 3
					sts.Status.CurrentRevision = "some-new-revision"
					sts.Status.UpdatedReplicas = 3
					sts.Status.UpdateRevision = "some-new-revision"
					sts.Status.ReadyReplicas = 2
					statusWriter := client.Status()
					err := statusWriter.Update(ctx, sts)
					Expect(err).NotTo(HaveOccurred())
					Eventually(func() map[string]string {
						rmq := &rabbitmqv1beta1.RabbitmqCluster{}
						err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
						Expect(err).To(BeNil())
						annotations = rmq.ObjectMeta.Annotations
						return annotations
					}, 5).Should(HaveKey("rabbitmq.com/queueRebalanceNeededAt"))
					Expect(fakeExecutor.ExecutedCommands()).NotTo(ContainElement(command{"sh", "-c", "rabbitmq-queues rebalance all"}))
					_, err = time.Parse(time.RFC3339, annotations["rabbitmq.com/queueRebalanceNeededAt"])
					Expect(err).NotTo(HaveOccurred(), "Annotation rabbitmq.com/queueRebalanceNeededAt was not a valid RFC3339 timestamp")
				})

				By("removing the annotation once all Pods are up, and triggering the queue rebalance", func() {
					sts.Status.ReadyReplicas = 3
					statusWriter := client.Status()
					err := statusWriter.Update(ctx, sts)
					Expect(err).NotTo(HaveOccurred())
					Eventually(func() map[string]string {
						rmq := &rabbitmqv1beta1.RabbitmqCluster{}
						err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
						Expect(err).To(BeNil())
						return rmq.ObjectMeta.Annotations
					}, 5).ShouldNot(HaveKey("rabbitmq.com/queueRebalanceNeededAt"))
					Expect(fakeExecutor.ExecutedCommands()).To(ContainElement(command{"sh", "-c", "rabbitmq-queues rebalance all"}))
				})
			})
		})
	})

	When("the cluster is not configured to run post-deploy steps", func() {
		BeforeEach(func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-three-no-post-deploy",
					Namespace: defaultNamespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas:            pointer.Int32Ptr(3),
					SkipPostDeploySteps: true,
				},
			}
			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)
		})
		When("the cluster is updated", func() {
			var sts *appsv1.StatefulSet

			BeforeEach(func() {
				sts = statefulSet(ctx, cluster)
				sts.Status.Replicas = 3
				sts.Status.CurrentReplicas = 2
				sts.Status.CurrentRevision = "some-old-revision"
				sts.Status.UpdatedReplicas = 1
				sts.Status.UpdateRevision = "some-new-revision"

				statusWriter := client.Status()
				err := statusWriter.Update(ctx, sts)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not trigger the controller to run rabbitmq-queues rebalance all", func() {
				By("never setting the annotation on the CR", func() {
					Consistently(func() map[string]string {
						rmq := &rabbitmqv1beta1.RabbitmqCluster{}
						err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
						Expect(err).To(BeNil())
						return rmq.ObjectMeta.Annotations
					}, 5).ShouldNot(HaveKey("rabbitmq.com/queueRebalanceNeededAt"))
				})

				By("not running the rebalance command once all nodes are up", func() {
					sts.Status.CurrentReplicas = 3
					sts.Status.CurrentRevision = "some-new-revision"
					sts.Status.UpdatedReplicas = 3
					sts.Status.UpdateRevision = "some-new-revision"
					sts.Status.ReadyReplicas = 3
					statusWriter := client.Status()
					err := statusWriter.Update(ctx, sts)
					Expect(err).NotTo(HaveOccurred())
					Consistently(func() map[string]string {
						rmq := &rabbitmqv1beta1.RabbitmqCluster{}
						err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
						Expect(err).To(BeNil())
						return rmq.ObjectMeta.Annotations
					}, 5).ShouldNot(HaveKey("rabbitmq.com/queueRebalanceNeededAt"))
					Expect(fakeExecutor.ExecutedCommands()).NotTo(ContainElement(command{"sh", "-c", "rabbitmq-queues rebalance all"}))
				})

			})

		})
	})

	When("the cluster is a single node cluster", func() {
		BeforeEach(func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-one-rebalance",
					Namespace: defaultNamespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas:            pointer.Int32Ptr(1),
					SkipPostDeploySteps: false,
				},
			}
			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)
		})
		When("the cluster is updated", func() {
			var sts *appsv1.StatefulSet

			BeforeEach(func() {
				sts = statefulSet(ctx, cluster)
				sts.Status.Replicas = 1
				sts.Status.CurrentReplicas = 1
				sts.Status.CurrentRevision = "some-old-revision"
				sts.Status.UpdatedReplicas = 0
				sts.Status.UpdateRevision = "some-new-revision"
				sts.Status.ReadyReplicas = 0

				statusWriter := client.Status()
				err := statusWriter.Update(ctx, sts)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not trigger the controller to run rabbitmq-queues rebalance all", func() {
				By("never setting the annotation on the CR", func() {
					Consistently(func() map[string]string {
						rmq := &rabbitmqv1beta1.RabbitmqCluster{}
						err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
						Expect(err).To(BeNil())
						return rmq.ObjectMeta.Annotations
					}, 5).ShouldNot(HaveKey("rabbitmq.com/queueRebalanceNeededAt"))
				})

				By("not running the rebalance command once all nodes are up", func() {
					sts.Status.CurrentReplicas = 1
					sts.Status.CurrentRevision = "some-new-revision"
					sts.Status.UpdatedReplicas = 1
					sts.Status.UpdateRevision = "some-new-revision"
					sts.Status.ReadyReplicas = 1
					statusWriter := client.Status()
					err := statusWriter.Update(ctx, sts)
					Expect(err).NotTo(HaveOccurred())
					Consistently(func() map[string]string {
						rmq := &rabbitmqv1beta1.RabbitmqCluster{}
						err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
						Expect(err).To(BeNil())
						return rmq.ObjectMeta.Annotations
					}, 5).ShouldNot(HaveKey("rabbitmq.com/queueRebalanceNeededAt"))
					Expect(fakeExecutor.ExecutedCommands()).NotTo(ContainElement(command{"sh", "-c", "rabbitmq-queues rebalance all"}))
				})

			})

		})
	})
})
