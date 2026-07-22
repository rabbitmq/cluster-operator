package controllers_test

import (
	"fmt"
	"time"

	rabbithole "github.com/michaelklishin/rabbit-hole/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	controllers "github.com/rabbitmq/cluster-operator/v2/internal/controller"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Deprecated Feature Controller", func() {
	var (
		cluster          *rabbitmqv1beta1.RabbitmqCluster
		defaultNamespace = "default"
		reconciler       *controllers.DeprecatedFeatureReconciler
	)

	const (
		interval = 1 * time.Second
	)

	BeforeEach(func() {
		cluster = &rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("rabbitmq-deprecated-features-%d", time.Now().Unix()),
				Namespace: defaultNamespace,
			},
		}
		Expect(client.Create(ctx, cluster)).To(Succeed())
		waitForClusterCreation(ctx, cluster, client)

		reconciler = &controllers.DeprecatedFeatureReconciler{
			Client:                client,
			APIReader:             client,
			RabbitmqClientFactory: fakeRabbitmqFactory,
			Interval:              interval,
		}
	})

	AfterEach(func() {
		Expect(client.Delete(ctx, cluster)).To(Succeed())
	})

	When("the client returns an error", func() {
		It("does not update status", func() {
			// Mock client should not be called
			fakeRabbitmqFactory.client = &fakeRabbitmqClient{
				err: fmt.Errorf("not quite there yet"),
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				},
			}

			// Always should requeue after the interval
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{RequeueAfter: interval}))

			// Status should remain empty because the client returned an error
			rmq := &rabbitmqv1beta1.RabbitmqCluster{}
			Expect(client.Get(ctx, req.NamespacedName, rmq)).To(Succeed())
			Expect(rmq.Status.DeprecatedFeaturesUsed).To(BeEmpty())
		})
	})

	When("the client returns the deprecated features", func() {
		It("updates the status with the deprecated features", func() {
			// Get the created StatefulSet and update it to be ready
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return client.Get(ctx, types.NamespacedName{
					Name:      cluster.ChildResourceName("server"),
					Namespace: cluster.Namespace,
				}, sts)
			}).Should(Succeed())

			sts.Spec.Replicas = new(int32(1))
			Expect(client.Update(ctx, sts)).To(Succeed())

			sts.Status.ReadyReplicas = 1
			sts.Status.Replicas = 1
			sts.Status.CurrentReplicas = 1
			sts.Status.UpdatedReplicas = 1
			sts.Status.CurrentRevision = "rev1"
			sts.Status.UpdateRevision = "rev1"
			Expect(client.Status().Update(ctx, sts)).To(Succeed())

			fakeRabbitmqFactory.client = &fakeRabbitmqClient{
				deprecatedFeatures: []rabbithole.DeprecatedFeature{
					{Name: "feature1"},
					{Name: "feature2"},
				},
				err: nil,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				},
			}

			Eventually(func() []string {
				_, err := reconciler.Reconcile(ctx, req)
				Expect(err).NotTo(HaveOccurred())

				updatedRabbit := &rabbitmqv1beta1.RabbitmqCluster{}
				Expect(client.Get(ctx, req.NamespacedName, updatedRabbit)).To(Succeed())
				return updatedRabbit.Status.DeprecatedFeaturesUsed
			}).
				Within(5 * time.Second).
				WithPolling(500 * time.Millisecond).
				Should(Equal([]string{"feature1", "feature2"}))
		})

		It("does not update status if features haven't changed", func() {
			// Get the created StatefulSet and update it to be ready
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return client.Get(ctx, types.NamespacedName{
					Name:      cluster.ChildResourceName("server"),
					Namespace: cluster.Namespace,
				}, sts)
			}).Should(Succeed())

			sts.Spec.Replicas = new(int32(1))
			Expect(client.Update(ctx, sts)).To(Succeed())

			sts.Status.ReadyReplicas = 1
			sts.Status.Replicas = 1
			sts.Status.CurrentReplicas = 1
			sts.Status.UpdatedReplicas = 1
			sts.Status.CurrentRevision = "rev1"
			sts.Status.UpdateRevision = "rev1"
			Expect(client.Status().Update(ctx, sts)).To(Succeed())

			// Initial setup with features
			rmq := &rabbitmqv1beta1.RabbitmqCluster{}
			Eventually(func() error {
				err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
				if err != nil {
					return err
				}
				patch := runtimeClient.MergeFrom(rmq.DeepCopy())
				rmq.Status.DeprecatedFeaturesUsed = []string{"feature1", "feature2"}
				return client.Status().Patch(ctx, rmq, patch)
			}).Should(Succeed())

			// Mock client returns same features
			fakeRabbitmqFactory.client = &fakeRabbitmqClient{
				deprecatedFeatures: []rabbithole.DeprecatedFeature{
					{Name: "feature1"},
					{Name: "feature2"},
				},
				err: nil,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				},
			}

			// Get the resource version before reconcile
			updatedRabbit := &rabbitmqv1beta1.RabbitmqCluster{}
			Expect(client.Get(ctx, req.NamespacedName, updatedRabbit)).To(Succeed())

			// Execute reconcile directly
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{RequeueAfter: interval}))

			// The status shouldn't change
			Consistently(func() []string {
				checkRabbit := &rabbitmqv1beta1.RabbitmqCluster{}
				Expect(client.Get(ctx, req.NamespacedName, checkRabbit)).To(Succeed())
				return checkRabbit.Status.DeprecatedFeaturesUsed
			}).
				Within(2 * time.Second).
				WithPolling(200 * time.Millisecond).
				Should(Equal([]string{"feature1", "feature2"}))
		})
	})
})
