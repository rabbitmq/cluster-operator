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
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const timeout = 3 * time.Second

var _ = Describe("RabbitmqclusterController", func() {

	var (
		rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster
		registrySecret  *corev1.Secret
		secretName      = "rabbitmq-one-registry-access"
	)

	var resourceTests = func() {
		It("reconciles", func() {
			By("creating the server conf configmap", func() {
				configMapName := rabbitmqCluster.ChildResourceName("server-conf")
				configMap, err := clientSet.CoreV1().ConfigMaps(rabbitmqCluster.Namespace).Get(configMapName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(configMap.Name).To(Equal(configMapName))
			})

			By("creating a rabbitmq admin secret", func() {
				secretName := rabbitmqCluster.ChildResourceName("admin")
				secret, err := clientSet.CoreV1().Secrets(rabbitmqCluster.Namespace).Get(secretName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Name).To(Equal(secretName))
			})

			By("creating an erlang cookie secret", func() {
				secretName := rabbitmqCluster.ChildResourceName("erlang-cookie")
				secret, err := clientSet.CoreV1().Secrets(rabbitmqCluster.Namespace).Get(secretName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Name).To(Equal(secretName))
			})

			By("creating a rabbitmq ingress service object", func() {
				ingressServiceName := rabbitmqCluster.ChildResourceName("ingress")
				service, err := clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(ingressServiceName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(service.Name).To(Equal(ingressServiceName))
			})

			By("creating a rabbitmq headless service object", func() {
				headlessServiceName := rabbitmqCluster.ChildResourceName("headless")
				service, err := clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(headlessServiceName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(service.Name).To(Equal(headlessServiceName))
			})

			By("creating a service account", func() {
				name := rabbitmqCluster.ChildResourceName("server")
				serviceAccount, err := clientSet.CoreV1().ServiceAccounts(rabbitmqCluster.Namespace).Get(name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(serviceAccount.Name).To(Equal(name))
			})

			By("creating a role", func() {
				name := rabbitmqCluster.ChildResourceName("endpoint-discovery")
				serviceAccount, err := clientSet.RbacV1().Roles(rabbitmqCluster.Namespace).Get(name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(serviceAccount.Name).To(Equal(name))
			})

			By("creating a role binding", func() {
				name := rabbitmqCluster.ChildResourceName("server")
				serviceAccount, err := clientSet.RbacV1().RoleBindings(rabbitmqCluster.Namespace).Get(name, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(serviceAccount.Name).To(Equal(name))
			})
		})
	}

	When("the imagePullSecret is specified only in config", func() {
		BeforeEach(func() {
			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-one",
					Namespace: "default",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: 1,
				},
			}

			registrySecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pivotal-rmq-registry-access",
					Namespace: "pivotal-rabbitmq-system",
				},
			}
			Expect(client.Create(context.TODO(), registrySecret)).To(Succeed())
			Expect(client.Create(context.TODO(), rabbitmqCluster)).NotTo(HaveOccurred())
			Eventually(func() string {
				rabbitmqClusterCreated := rabbitmqv1beta1.RabbitmqCluster{}
				err := client.Get(
					context.TODO(),
					types.NamespacedName{Name: rabbitmqCluster.Name, Namespace: rabbitmqCluster.Namespace},
					&rabbitmqClusterCreated,
				)
				if err != nil {
					return fmt.Sprintf("%v+", err)
				}

				return rabbitmqClusterCreated.Status.ClusterStatus

			}, 5, 1).Should(ContainSubstring("created"))

		})
		AfterEach(func() {
			Expect(client.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
			Expect(client.Delete(context.TODO(), registrySecret)).To(Succeed())
			Expect(clientSet.CoreV1().Secrets(rabbitmqCluster.Namespace).Delete(secretName, &metav1.DeleteOptions{})).To(Succeed())
		})

		It("creating the registry secret", func() {
			Eventually(func() *corev1.Secret {
				secret, _ := clientSet.CoreV1().Secrets(rabbitmqCluster.Namespace).Get(secretName, metav1.GetOptions{})
				return secret
			}).ShouldNot(BeNil())
			stsName := rabbitmqCluster.ChildResourceName("server")
			sts, err := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(stsName, metav1.GetOptions{})
			Expect(sts.Spec.Template.Spec.ImagePullSecrets).To(ContainElement(corev1.LocalObjectReference{Name: secretName}))
			Expect(err).NotTo(HaveOccurred())
			Expect(sts.Name).To(Equal(stsName))

		})
		resourceTests()
	})

	When("the imagePullSecret is specified in the instance spec (and config)", func() {
		BeforeEach(func() {
			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-two",
					Namespace: "default",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas:        1,
					ImagePullSecret: "rabbit-two-secret",
				},
			}

			registrySecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pivotal-rmq-registry-access",
					Namespace: "pivotal-rabbitmq-system",
				},
			}
			Expect(client.Create(context.TODO(), registrySecret)).To(Succeed())
			Expect(client.Create(context.TODO(), rabbitmqCluster)).NotTo(HaveOccurred())
			Eventually(func() string {
				rabbitmqClusterCreated := rabbitmqv1beta1.RabbitmqCluster{}
				err := client.Get(
					context.TODO(),
					types.NamespacedName{Name: rabbitmqCluster.Name, Namespace: rabbitmqCluster.Namespace},
					&rabbitmqClusterCreated,
				)
				if err != nil {
					return fmt.Sprintf("%v+", err)
				}

				return rabbitmqClusterCreated.Status.ClusterStatus

			}, 5, 1).Should(ContainSubstring("created"))
		})
		AfterEach(func() {
			Expect(client.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
			Expect(client.Delete(context.TODO(), registrySecret)).To(Succeed())
		})

		It("does not create a new registry secret", func() {
			stsName := rabbitmqCluster.ChildResourceName("server")
			var sts *appsv1.StatefulSet
			Eventually(func() *appsv1.StatefulSet {
				sts, _ = clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(stsName, metav1.GetOptions{})
				return sts
			}).ShouldNot(BeNil())
			Expect(sts.Spec.Template.Spec.ImagePullSecrets).To(ContainElement(corev1.LocalObjectReference{Name: "rabbit-two-secret"}))
			Expect(sts.Name).To(Equal(stsName))

			imageSecretSuffix := "registry-access"
			secretList, err := clientSet.CoreV1().Secrets(rabbitmqCluster.Namespace).List(metav1.ListOptions{})
			var secretsWithImagePullSecretSuffix []corev1.Secret
			for _, i := range secretList.Items {
				if strings.Contains(i.Name, imageSecretSuffix) {
					secretsWithImagePullSecretSuffix = append(secretsWithImagePullSecretSuffix, i)
				}
			}
			Expect(secretsWithImagePullSecretSuffix).To(BeEmpty())
			Expect(err).NotTo(HaveOccurred())
		})
		resourceTests()
	})
})
