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
			rabbitmqCluster   *rabbitmqv1beta1.RabbitmqCluster
			expectedRequest   reconcile.Request
			clientSet         *kubernetes.Clientset
			stsName           = "p-foo"
			configMapBaseName = "rabbitmq-default-plugins"
			configMapName     string
			secretName        = "foo-rabbitmq-secret"
		)

		BeforeEach(func() {
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

			var err error
			configMapName = rabbitmqCluster.Name + "-" + configMapBaseName
			Expect(client.Create(context.TODO(), rabbitmqCluster)).NotTo(HaveOccurred())
			clientSet, err = kubernetes.NewForConfig(cfg)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := client.Delete(context.TODO(), rabbitmqCluster)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("not found"))
			}

		})

		It("creates the StatefulSet", func() {
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			sts, err := clientSet.AppsV1().StatefulSets("default").Get(stsName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(sts.Name).To(Equal(stsName))
		})

		It("creates the configmap", func() {
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			configMap, err := clientSet.CoreV1().ConfigMaps("default").Get(configMapName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(configMap.Name).To(Equal(configMapName))
		})

		It("creates a rabbitmq secret object", func() {
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			secret, err := clientSet.CoreV1().Secrets("default").Get(secretName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Name).To(Equal(secretName))
		})
		Context("Using a second RabbitmqCluster", func() {

			var (
				rabbitmqClusterRabbit2 *rabbitmqv1beta1.RabbitmqCluster
				clientSetRabbit2       *kubernetes.Clientset
				expectedRequest2       reconcile.Request
				configMapNameRabbit2   string
			)

			BeforeEach(func() {
				// Create second cluster name rabbit2
				expectedRequest2 = reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "rabbit2", Namespace: "default",
					},
				}

				rabbitmqClusterRabbit2 = &rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rabbit2",
						Namespace: "default",
					},
					Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
						Plan: "single",
					},
				}

				var err error
				Expect(client.Create(context.TODO(), rabbitmqClusterRabbit2)).NotTo(HaveOccurred())
				clientSetRabbit2, err = kubernetes.NewForConfig(cfg)
				Expect(err).NotTo(HaveOccurred())

				configMapNameRabbit2 = rabbitmqClusterRabbit2.Name + "-" + configMapBaseName
			})

			AfterEach(func() {
				Expect(client.Delete(context.TODO(), rabbitmqClusterRabbit2)).NotTo(HaveOccurred())
			})

			It("creates two ConfigMaps and deletes one rabbitmqCluster", func() {
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequest2)))

				_, err := clientSetRabbit2.CoreV1().ConfigMaps("default").Get(configMapNameRabbit2, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				err = client.Delete(context.TODO(), rabbitmqCluster)
				Expect(err).NotTo(HaveOccurred())

				_, err = clientSetRabbit2.CoreV1().ConfigMaps("default").Get(configMapNameRabbit2, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
