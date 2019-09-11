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

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
)

const timeout = time.Second

var _ = Describe("RabbitmqclusterController", func() {

	Context("when Reconcile is called", func() {
		var (
			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-one",
					Namespace: "default",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: 1,
				},
			}

			expectedRequest = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: "rabbitmq-one", Namespace: "default",
				},
			}
		)

		BeforeEach(func() {
			Expect(client.Create(context.TODO(), rabbitmqCluster)).NotTo(HaveOccurred())
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
		})

		AfterEach(func() {
			Expect(client.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
		})

		It("works", func() {
			By("creating the StatefulSet", func() {
				stsName := rabbitmqCluster.ChildResourceName("rabbitmq-server")
				sts, err := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(stsName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(sts.Name).To(Equal(stsName))
			})

			By("creating the plugins configmap", func() {
				configMapName := rabbitmqCluster.ChildResourceName("rabbitmq-plugins")
				configMap, err := clientSet.CoreV1().ConfigMaps(rabbitmqCluster.Namespace).Get(configMapName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(configMap.Name).To(Equal(configMapName))
			})

			By("creating the plugins configmap", func() {
				configMapName := rabbitmqCluster.ChildResourceName("rabbitmq-conf")
				configMap, err := clientSet.CoreV1().ConfigMaps(rabbitmqCluster.Namespace).Get(configMapName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(configMap.Name).To(Equal(configMapName))
			})

			By("creating a rabbitmq secret object", func() {
				secretName := rabbitmqCluster.ChildResourceName("rabbitmq-admin")
				secret, err := clientSet.CoreV1().Secrets(rabbitmqCluster.Namespace).Get(secretName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Name).To(Equal(secretName))
			})

			By("creating a rabbitmq ingress service object", func() {
				ingressServiceName := rabbitmqCluster.ChildResourceName("rabbitmq-ingress")
				service, err := clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(ingressServiceName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(service.Name).To(Equal(ingressServiceName))
			})

			By("creating a rabbitmq headless service object", func() {
				headlessServiceName := rabbitmqCluster.ChildResourceName("rabbitmq-headless")
				service, err := clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(headlessServiceName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(service.Name).To(Equal(headlessServiceName))
			})

			By("creating a service account", func() {
				name := rabbitmqCluster.ChildResourceName("rabbitmq-server")
				serviceAccount, err := clientSet.CoreV1().ServiceAccounts(rabbitmqCluster.Namespace).Get(name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(serviceAccount.Name).To(Equal(name))
			})

			By("creating a role", func() {
				name := rabbitmqCluster.ChildResourceName("rabbitmq-endpoint-discovery")
				serviceAccount, err := clientSet.RbacV1().Roles(rabbitmqCluster.Namespace).Get(name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(serviceAccount.Name).To(Equal(name))
			})

			By("creating a role binding", func() {
				name := rabbitmqCluster.ChildResourceName("rabbitmq-server")
				serviceAccount, err := clientSet.RbacV1().RoleBindings(rabbitmqCluster.Namespace).Get(name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(serviceAccount.Name).To(Equal(name))
			})
		})
	})
})
