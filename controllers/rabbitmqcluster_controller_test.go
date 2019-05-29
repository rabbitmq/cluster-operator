/*
Copyright 2019 Pivotal.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers_test

import (
	"context"
	"sync"
	"time"

	"github.com/pivotal/rabbitmq-for-kubernetes/controllers"

	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"

	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("RabbitmqclusterController", func() {
	Context("Reconcile", func() {
		var stopMgr chan struct{}
		var mgrStopped *sync.WaitGroup
		var client runtimeClient.Client
		var requests chan reconcile.Request
		var rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster
		var expectedRequest reconcile.Request
		var stsName types.NamespacedName
		const timeout = time.Millisecond * 700

		BeforeEach(func() {
			stsName = types.NamespacedName{Name: "p-foo", Namespace: "default"}

			expectedRequest = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: "foo", Namespace: "default",
				},
			}

			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Plan: "single",
				},
			}

			// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
			// channel when it is finished.
			mgr, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())
			client = mgr.GetClient()

			err = (&controllers.RabbitmqClusterReconciler{
				Client: mgr.GetClient(),
				Log:    ctrl.Log.WithName("controllers").WithName("RabbitmqCluster"),
				Scheme: mgr.GetScheme(),
			}).SetupWithManager(mgr)

			Expect(err).NotTo(HaveOccurred())

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

		It("Creates a one pod sts when plan is set to single", func() {
			// Create the RabbitmqCluster object and expect the Reconcile to be created
			err := client.Create(context.TODO(), rabbitmqCluster)
			Expect(err).NotTo(HaveOccurred())
			defer client.Delete(context.TODO(), rabbitmqCluster)

			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			clientSet, err := kubernetes.NewForConfig(cfg)
			Expect(err).NotTo(HaveOccurred())

			sts, err := clientSet.AppsV1().StatefulSets("default").Get(stsName.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(sts.Name).To(Equal(stsName.Name))

		})
	})
})
