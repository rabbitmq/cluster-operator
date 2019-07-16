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

const timeout = time.Second

var _ = Describe("RabbitmqclusterController", func() {

	Context("when Reconcile is called", func() {
		var (
			rabbitmqClusterOne    *rabbitmqv1beta1.RabbitmqCluster
			expectedRequestForOne reconcile.Request
			clientSetOne          *kubernetes.Clientset
			stsName               = "p-rabbitmq-one"
			configMapBaseName     = "rabbitmq-default-plugins"
			configMapName         string
			secretName            = "rabbitmq-one-rabbitmq-secret"
			serviceName           = "p-rabbitmq-one"
		)

		BeforeEach(func() {
			expectedRequestForOne = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: "rabbitmq-one", Namespace: "default",
				},
			}

			rabbitmqClusterOne = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-one",
					Namespace: "default",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Plan: "single",
				},
			}

			var err error
			configMapName = rabbitmqClusterOne.Name + "-" + configMapBaseName
			Expect(client.Create(context.TODO(), rabbitmqClusterOne)).NotTo(HaveOccurred())
			clientSetOne, err = kubernetes.NewForConfig(cfg)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := client.Delete(context.TODO(), rabbitmqClusterOne)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("not found"))
			}

		})

		It("creates the StatefulSet", func() {
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequestForOne)))

			sts, err := clientSetOne.AppsV1().StatefulSets("default").Get(stsName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(sts.Name).To(Equal(stsName))
		})

		It("creates the configmap", func() {
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequestForOne)))

			configMap, err := clientSetOne.CoreV1().ConfigMaps("default").Get(configMapName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(configMap.Name).To(Equal(configMapName))
		})

		It("creates a rabbitmq secret object", func() {
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequestForOne)))

			secret, err := clientSetOne.CoreV1().Secrets("default").Get(secretName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Name).To(Equal(secretName))
		})

		It("creates a rabbitmq service object", func() {
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequestForOne)))

			service, err := clientSetOne.CoreV1().Services("default").Get(serviceName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(service.Name).To(Equal(serviceName))
		})

		Context("using a second RabbitmqCluster", func() {

			var (
				rabbitmqClusterTwo    *rabbitmqv1beta1.RabbitmqCluster
				clientSetForTwo       *kubernetes.Clientset
				expectedRequestForTwo reconcile.Request
				configMapNameTwo      string
			)

			BeforeEach(func() {
				expectedRequestForTwo = reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "rabbitmq-two", Namespace: "default",
					},
				}

				rabbitmqClusterTwo = &rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rabbitmq-two",
						Namespace: "default",
					},
					Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
						Plan: "single",
					},
				}

				var err error
				Expect(client.Create(context.TODO(), rabbitmqClusterTwo)).NotTo(HaveOccurred())
				clientSetForTwo, err = kubernetes.NewForConfig(cfg)
				Expect(err).NotTo(HaveOccurred())

				configMapNameTwo = rabbitmqClusterTwo.Name + "-" + configMapBaseName
			})

			AfterEach(func() {
				Expect(client.Delete(context.TODO(), rabbitmqClusterTwo)).NotTo(HaveOccurred())
			})

			It("deletes rabbitmqClusterOne and finds configMap for rabbitmqClusterTwo", func() {
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequestForOne)))
				Eventually(requests, timeout).Should(Receive(Equal(expectedRequestForTwo)))

				_, err := clientSetForTwo.CoreV1().ConfigMaps("default").Get(configMapNameTwo, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				err = client.Delete(context.TODO(), rabbitmqClusterOne)
				Expect(err).NotTo(HaveOccurred())

				_, err = clientSetForTwo.CoreV1().ConfigMaps("default").Get(configMapNameTwo, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Context("using a private container image", func() {
		var rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster
		var err error
		var expectedRequest reconcile.Request
		var clientSet *kubernetes.Clientset
		var namespace, instanceName, stsName, rabbitmqManagementImage string

		BeforeEach(func() {
			instanceName = "rabbitmq"
			stsName = "p-" + instanceName
			namespace = "default"
			rabbitmqManagementImage = "rabbitmq:3.8-rc-management@sha256:5d49702395e2c0cf9fa77a46d8fcae5f2107dc9a6dcd87a2c27e0599eba20d18"

			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instanceName,
					Namespace: namespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Plan: "single",
					Image: rabbitmqv1beta1.RabbitmqClusterImageSpec{
						Repository: "my-private-repo",
					},
					ImagePullSecret: "my-best-secret",
				},
			}

			expectedRequest = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: "rabbitmq", Namespace: "default",
				},
			}

			Expect(client.Create(context.TODO(), rabbitmqCluster)).NotTo(HaveOccurred())
			clientSet, err = kubernetes.NewForConfig(cfg)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(client.Delete(context.TODO(), rabbitmqCluster)).NotTo(HaveOccurred())
		})

		It("templates the Stateful Set with the specified ImagePullSecrets", func() {
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
			sts, err := clientSet.AppsV1().StatefulSets(namespace).Get(stsName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(sts.Spec.Template.Spec.ImagePullSecrets[0].Name).To(Equal("my-best-secret"))
		})

		It("templates the Stateful Set with the specified private repository", func() {
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
			sts, err := clientSet.AppsV1().StatefulSets(namespace).Get(stsName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(sts.Spec.Template.Spec.Containers[0].Image).To(Equal("my-private-repo/" + rabbitmqManagementImage))
		})

	})

})
