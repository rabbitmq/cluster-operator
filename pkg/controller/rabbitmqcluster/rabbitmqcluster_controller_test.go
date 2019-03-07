package rabbitmqcluster_test

import (
	"context"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/pkg/apis/rabbitmq/v1beta1"
	. "github.com/pivotal/rabbitmq-for-kubernetes/pkg/controller/rabbitmqcluster"
)

var _ = Describe("RabbitmqclusterController", func() {
	Context("Reconcile", func() {
		var stopMgr chan struct{}
		var mgrStopped *sync.WaitGroup
		var client client.Client
		var requests chan reconcile.Request
		var recFn reconcile.Reconciler
		var instance *rabbitmqv1beta1.RabbitmqCluster
		var expectedRequest reconcile.Request
		var depKey types.NamespacedName
		const timeout = time.Millisecond * 500

		BeforeEach(func() {
			expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
			depKey = types.NamespacedName{Name: "foo-rabbitmq", Namespace: "default"}

			nodes := int32(3)
			instance = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Nodes: nodes,
				},
			}

			// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
			// channel when it is finished.
			mgr, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())
			client = mgr.GetClient()

			recFn, requests = SetupTestReconcile(NewReconciler(mgr))
			Expect(AddController(mgr, recFn)).NotTo(HaveOccurred())

			stopMgr = make(chan struct{})
			mgrStopped = &sync.WaitGroup{}
			mgrStopped.Add(1)
			go func() {
				defer mgrStopped.Done()
				Expect(mgr.Start(stopMgr)).NotTo(HaveOccurred())
			}()

		})

		AfterEach(func() {
			close(stopMgr)
			mgrStopped.Wait()
		})

		It("Manages the lifecycle of the cluster and its dependencies", func() {

			// Create the RabbitmqCluster object and expect the Reconcile and Service to be created
			err := client.Create(context.TODO(), instance)
			Expect(err).NotTo(HaveOccurred())
			defer client.Delete(context.TODO(), instance)

			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			service := &v1.Service{}
			Eventually(func() error { return client.Get(context.TODO(), depKey, service) }, timeout).
				Should(Succeed())

			// Delete the service and expect Reconcile to be called to recreate it
			Expect(client.Delete(context.TODO(), service)).NotTo(HaveOccurred())
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
			Eventually(func() error { return client.Get(context.TODO(), depKey, service) }, timeout).
				Should(Succeed())

			// Manually delete service since GC isn't enabled in the test control plane
			Eventually(func() error { return client.Delete(context.TODO(), service) }, timeout).
				Should(MatchError("services \"foo-rabbitmq\" not found"))
		})
	})

})
