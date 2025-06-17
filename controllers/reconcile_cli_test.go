package controllers_test

import (
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
)

var _ = Describe("Reconcile CLI", func() {
	var (
		cluster          *rabbitmqv1beta1.RabbitmqCluster
		defaultNamespace = "default"
	)

	AfterEach(func() {
		Expect(client.Delete(ctx, cluster)).To(Succeed())
		waitForClusterDeletion(ctx, cluster, client)
	})

	When("cluster is created", func() {
		var sts *appsv1.StatefulSet
		BeforeEach(func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-feature-flags",
					Namespace: defaultNamespace,
				},
			}
			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)
		})

		It("enables all feature flags", func() {
			By("annotating that StatefulSet got created", func() {
				sts = statefulSet(ctx, cluster)
				value := sts.Annotations["rabbitmq.com/createdAt"]
				_, err := time.Parse(time.RFC3339, value)
				Expect(err).NotTo(HaveOccurred(), "annotation rabbitmq.com/createdAt was not a valid RFC3339 timestamp")
			})

			By("enabling all feature flags once all Pods are up, and removing the annotation", func() {
				sts.Status.Replicas = 1
				sts.Status.ReadyReplicas = 1
				Expect(client.Status().Update(ctx, sts)).To(Succeed())
				Eventually(func() map[string]string {
					Expect(client.Get(ctx, types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.ChildResourceName("server")}, sts)).To(Succeed())
					return sts.ObjectMeta.Annotations
				}, 5).ShouldNot(HaveKey("rabbitmq.com/createdAt"))
				Expect(fakeExecutor.ExecutedCommands()).To(ContainElement(command{"bash", "-c",
					"rabbitmqctl enable_feature_flag all"}))
			})
		})
	})

	When("the cluster is configured to run post-deploy steps", func() {
		BeforeEach(func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-three",
					Namespace: defaultNamespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: ptr.To(int32(3)),
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

				Expect(client.Status().Update(ctx, sts)).To(Succeed())
			})

			It("triggers the controller to run rabbitmq-queues rebalance all", func() {
				k := komega.New(client)

				By("setting an annotation on the CR", func() {
					rmq := &rabbitmqv1beta1.RabbitmqCluster{}
					rmq.Name = "rabbitmq-three"
					rmq.Namespace = defaultNamespace
					Eventually(k.Object(rmq)).Within(time.Second * 5).WithPolling(time.Second).Should(HaveField("ObjectMeta.Annotations", HaveKey("rabbitmq.com/queueRebalanceNeededAt")))

					_, err := time.Parse(time.RFC3339, rmq.Annotations["rabbitmq.com/queueRebalanceNeededAt"])
					Expect(err).NotTo(HaveOccurred(), "annotation rabbitmq.com/queueRebalanceNeededAt was not a valid RFC3339 timestamp")
				})

				By("not removing the annotation when all replicas are updated but not yet ready", func() {
					// Setup transition
					Eventually(k.UpdateStatus(sts, func() {
						sts.Status.CurrentReplicas = 3
						sts.Status.CurrentRevision = "some-new-revision"
						sts.Status.UpdatedReplicas = 3
						sts.Status.UpdateRevision = "some-new-revision"
						sts.Status.ReadyReplicas = 2
					})).Should(Succeed())

					// by not removing the annotation
					rmq := &rabbitmqv1beta1.RabbitmqCluster{}
					rmq.Name = "rabbitmq-three"
					rmq.Namespace = defaultNamespace
					Eventually(k.Object(rmq)).Within(time.Second * 5).WithPolling(time.Second).Should(HaveField("ObjectMeta.Annotations", HaveKey("rabbitmq.com/queueRebalanceNeededAt")))

					// by not running the commands
					Expect(fakeExecutor.ExecutedCommands()).NotTo(ContainElement(command{"sh", "-c", "rabbitmq-queues rebalance all"}))
					Expect(fakeExecutor.ExecutedCommands()).NotTo(ContainElement(command{"bash", "-c", "rabbitmqctl enable_feature_flag all"}))
					_, err := time.Parse(time.RFC3339, rmq.Annotations["rabbitmq.com/queueRebalanceNeededAt"])
					Expect(err).NotTo(HaveOccurred(), "Annotation rabbitmq.com/queueRebalanceNeededAt was not a valid RFC3339 timestamp")
				})

				By("removing the annotation once all Pods are up, and triggering the queue rebalance", func() {
					// setup transition to all pods ready
					sts.Status.ReadyReplicas = 3
					Expect(client.Status().Update(ctx, sts)).To(Succeed())

					// by not having the annotation
					rmq := &rabbitmqv1beta1.RabbitmqCluster{}
					rmq.Name = "rabbitmq-three"
					rmq.Namespace = defaultNamespace
					Eventually(k.Object(rmq)).Within(time.Second * 5).WithPolling(time.Second).ShouldNot(HaveField("ObjectMeta.Annotations", HaveKey("rabbitmq.com/queueRebalanceNeededAt")))

					// by executing the commands
					Expect(fakeExecutor.ExecutedCommands()).To(ContainElement(command{"sh", "-c", "rabbitmq-queues rebalance all"}))
					Expect(fakeExecutor.ExecutedCommands()).To(ContainElement(command{"bash", "-c", "rabbitmqctl enable_feature_flag all"}))
				})
			})
		})

		Describe("autoEnableAllFeatureFlags", func() {
			var (
				sts *appsv1.StatefulSet
				k   komega.Komega
			)

			BeforeEach(func() {
				k = komega.New(client)
				sts = statefulSet(ctx, cluster)
				sts.Status = appsv1.StatefulSetStatus{
					Replicas:          3,
					ReadyReplicas:     3,
					CurrentReplicas:   3,
					UpdatedReplicas:   3,
					CurrentRevision:   "the last one",
					UpdateRevision:    "the last one",
					AvailableReplicas: 3,
				}
				sts.Annotations = make(map[string]string)
				Expect(client.Status().Update(ctx, sts)).To(Succeed())
			})

			When("disabled", func() {
				It("doesn't call enable_feature_flag CLI", func() {
					// BeforeEach updated the STS from zero replicas ready to all ready
					// Initial rabbitmqcluster create command has reconciles pending to have
					// all replicas ready to execute commands. We have to reset the "registry"
					// of commands.
					fakeExecutor.ResetExecutedCommands()

					Eventually(k.Update(cluster, func() {
						if cluster != nil {
							cluster.Spec.AutoEnableAllFeatureFlags = false
						}
					})).Should(Succeed())
					Consistently(fakeExecutor.ExecutedCommands).Within(time.Second * 5).WithPolling(time.Second).
						ShouldNot(ContainElement(command{"bash", "-c", "rabbitmqctl enable_feature_flag all"}))
				})
			})

			When("enabled", func() {
				It("calls enable_feature_flag CLI", func() {
					// BeforeEach updated the STS from zero replicas ready to all ready
					// Initial rabbitmqcluster create command has reconciles pending to have
					// all replicas ready to execute commands. We have to reset the "registry"
					// of commands.
					fakeExecutor.ResetExecutedCommands()

					Eventually(k.Update(cluster, func() {
						if cluster != nil {
							cluster.Spec.AutoEnableAllFeatureFlags = true
						}
					})).Should(Succeed())
					Eventually(fakeExecutor.ExecutedCommands).Within(time.Second * 5).WithPolling(time.Second).
						Should(ContainElement(command{"bash", "-c", "rabbitmqctl enable_feature_flag all"}))
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
					Replicas:            ptr.To(int32(3)),
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

				Expect(client.Status().Update(ctx, sts)).To(Succeed())
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
					Expect(client.Status().Update(ctx, sts)).To(Succeed())
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
					Replicas:            ptr.To(int32(1)),
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

				Expect(client.Status().Update(ctx, sts)).To(Succeed())
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
					Expect(client.Status().Update(ctx, sts)).To(Succeed())
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
