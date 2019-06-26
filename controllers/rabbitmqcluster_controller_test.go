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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
)

var _ = Describe("RabbitmqclusterController", func() {
	Context("when Reconcile is called", func() {
		const timeout = time.Millisecond * 700
		var (
			rabbitmqClusterFoo    *rabbitmqv1beta1.RabbitmqCluster
			expectedRequestForFoo reconcile.Request
			clientSetFoo          *kubernetes.Clientset
			stsName               = "p-foo"
			configMapBaseName     = "rabbitmq-default-plugins"
			configMapName         string
			secretName            = "foo-rabbitmq-secret"
		)

		BeforeEach(func() {
			expectedRequestForFoo = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: "foo", Namespace: "default",
				},
			}

			rabbitmqClusterFoo = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Plan: "single",
				},
			}

			var err error
			configMapName = rabbitmqClusterFoo.Name + "-" + configMapBaseName
			Expect(client.Create(context.TODO(), rabbitmqClusterFoo)).NotTo(HaveOccurred())
			clientSetFoo, err = kubernetes.NewForConfig(cfg)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := client.Delete(context.TODO(), rabbitmqClusterFoo)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("not found"))
			}

		})

		It("creates the StatefulSet", func() {
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequestForFoo)))

			sts, err := clientSetFoo.AppsV1().StatefulSets("default").Get(stsName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(sts.Name).To(Equal(stsName))
		})

		It("creates the configmap", func() {
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequestForFoo)))

			configMap, err := clientSetFoo.CoreV1().ConfigMaps("default").Get(configMapName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(configMap.Name).To(Equal(configMapName))
		})

		It("creates a rabbitmq secret object", func() {
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequestForFoo)))

			secret, err := clientSetFoo.CoreV1().Secrets("default").Get(secretName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Name).To(Equal(secretName))
		})
		Context("Using a second RabbitmqCluster", func() {

			var (
				rabbitmqClusterBar    *rabbitmqv1beta1.RabbitmqCluster
				clientSetBar          *kubernetes.Clientset
				expectedRequestForBar reconcile.Request
				configMapNameBar      string
			)

			BeforeEach(func() {
				// Create second cluster name bar
				expectedRequestForBar = reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "bar", Namespace: "default",
					},
				}

				rabbitmqClusterBar = &rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bar",
						Namespace: "default",
					},
					Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
						Plan: "single",
					},
				}

				var err error
				Expect(client.Create(context.TODO(), rabbitmqClusterBar)).NotTo(HaveOccurred())
				clientSetBar, err = kubernetes.NewForConfig(cfg)
				Expect(err).NotTo(HaveOccurred())

				configMapNameBar = rabbitmqClusterBar.Name + "-" + configMapBaseName
			})

			AfterEach(func() {
				Expect(client.Delete(context.TODO(), rabbitmqClusterBar)).NotTo(HaveOccurred())
			})

			It("deletes rabbitmqClusterFoo and finds configMap bar", func() {
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequestForFoo)))
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequestForBar)))

				_, err := clientSetBar.CoreV1().ConfigMaps("default").Get(configMapNameBar, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				err = client.Delete(context.TODO(), rabbitmqClusterFoo)
				Expect(err).NotTo(HaveOccurred())

				_, err = clientSetBar.CoreV1().ConfigMaps("default").Get(configMapNameBar, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
