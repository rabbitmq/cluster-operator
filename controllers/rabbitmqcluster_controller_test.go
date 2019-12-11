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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("RabbitmqclusterController", func() {

	var (
		rabbitmqCluster        *rabbitmqv1beta1.RabbitmqCluster
		operatorRegistrySecret *corev1.Secret
		secretName             = "rabbitmq-one-registry-access"
		scheme                 *runtime.Scheme
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(rabbitmqv1beta1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(defaultscheme.AddToScheme(scheme)).NotTo(HaveOccurred())
	})

	Context("Custom Resource updates", func() {
		var (
			ingressServiceName string
			statefulSetName    string
		)
		BeforeEach(func() {
			operatorRegistrySecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pivotal-rmq-registry-access",
					Namespace: "pivotal-rabbitmq-system",
				},
			}
			Expect(client.Create(context.TODO(), operatorRegistrySecret)).To(Succeed())

			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-two",
					Namespace: "rabbitmq-two",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas:        1,
					ImagePullSecret: "rabbit-two-secret",
				},
			}
			ingressServiceName = rabbitmqCluster.ChildResourceName("ingress")
			statefulSetName = rabbitmqCluster.ChildResourceName("server")

			Expect(client.Create(context.TODO(), rabbitmqCluster)).To(Succeed())
			waitForClusterCreation(rabbitmqCluster, client)
		})

		AfterEach(func() {
			Expect(client.Delete(context.TODO(), operatorRegistrySecret)).To(Succeed())
			Expect(client.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
		})

		It("reconciles an existing instance", func() {
			Expect(client.Get(
				context.TODO(),
				types.NamespacedName{Name: rabbitmqCluster.Name, Namespace: rabbitmqCluster.Namespace},
				rabbitmqCluster,
			)).To(Succeed())

			When("the service annotations are updated", func() {
				rabbitmqCluster.Spec.Service.Annotations = map[string]string{"test-key": "test-value"}
				Expect(client.Update(context.TODO(), rabbitmqCluster)).To(Succeed())
				Eventually(func() map[string]string {
					ingressServiceName := rabbitmqCluster.ChildResourceName("ingress")
					service, _ := clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(ingressServiceName, metav1.GetOptions{})
					return service.Annotations
				}, 10).Should(HaveKeyWithValue("test-key", "test-value"))
			})

			When("the CPU and memory requirements are updated", func() {
				var resourceRequirements corev1.ResourceRequirements
				expectedRequirements := corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    k8sresource.MustParse("1100m"),
						corev1.ResourceMemory: k8sresource.MustParse("5Gi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    k8sresource.MustParse("1200m"),
						corev1.ResourceMemory: k8sresource.MustParse("6Gi"),
					},
				}
				rabbitmqCluster.Spec.Resource.Request.CPU = "1100m"
				rabbitmqCluster.Spec.Resource.Request.Memory = "5Gi"
				rabbitmqCluster.Spec.Resource.Limit.CPU = "1200m"
				rabbitmqCluster.Spec.Resource.Limit.Memory = "6Gi"
				Expect(client.Update(context.TODO(), rabbitmqCluster)).To(Succeed())

				Eventually(func() corev1.ResourceList {
					stsName := rabbitmqCluster.ChildResourceName("server")
					sts, err := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(stsName, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					resourceRequirements = sts.Spec.Template.Spec.Containers[0].Resources
					return resourceRequirements.Requests
				}, 100).Should(HaveKeyWithValue(corev1.ResourceCPU, expectedRequirements.Requests[corev1.ResourceCPU]))
				Expect(resourceRequirements.Limits).To(HaveKeyWithValue(corev1.ResourceCPU, expectedRequirements.Limits[corev1.ResourceCPU]))

				Expect(resourceRequirements.Requests).To(HaveKeyWithValue(corev1.ResourceMemory, expectedRequirements.Requests[corev1.ResourceMemory]))
				Expect(resourceRequirements.Limits).To(HaveKeyWithValue(corev1.ResourceMemory, expectedRequirements.Limits[corev1.ResourceMemory]))
			})

			When("CR labels are added", func() {
				labels := make(map[string]string)
				rabbitmqCluster.Labels = labels
				rabbitmqCluster.Labels["foo"] = "bar"
				Expect(client.Update(context.TODO(), rabbitmqCluster)).To(Succeed())

				Eventually(func() map[string]string {
					service, err := clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(ingressServiceName, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					return service.Labels
				}, 5).Should(HaveKeyWithValue("foo", "bar"))

				Eventually(func() map[string]string {
					sts, err := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(statefulSetName, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					return sts.Labels
				}, 10).Should(HaveKeyWithValue("foo", "bar"))
			})
		})
	})

	Context("ImagePullSecret", func() {
		BeforeEach(func() {
			scheme := runtime.NewScheme()
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).NotTo(HaveOccurred())
			Expect(defaultscheme.AddToScheme(scheme)).NotTo(HaveOccurred())

			operatorRegistrySecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pivotal-rmq-registry-access",
					Namespace: "pivotal-rabbitmq-system",
				},
			}
			Expect(client.Create(context.TODO(), operatorRegistrySecret)).To(Succeed())
		})

		AfterEach(func() {
			Expect(client.Delete(context.TODO(), operatorRegistrySecret)).To(Succeed())
		})

		When("specified in config", func() {
			BeforeEach(func() {
				rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rabbitmq-one",
						Namespace: "rabbitmq-one",
					},
					Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
						Replicas: 1,
					},
				}

				Expect(client.Create(context.TODO(), rabbitmqCluster)).NotTo(HaveOccurred())
				waitForClusterCreation(rabbitmqCluster, client)

			})
			AfterEach(func() {
				Expect(client.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
			})

			It("creates the registry secret", func() {
				Eventually(func() error {
					_, err := clientSet.CoreV1().Secrets(rabbitmqCluster.Namespace).Get(secretName, metav1.GetOptions{})
					return err
				}, 5).Should(Succeed())

				stsName := rabbitmqCluster.ChildResourceName("server")
				var sts *appsv1.StatefulSet
				Eventually(func() error {
					var err error
					sts, err = clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(stsName, metav1.GetOptions{})
					return err
				}, 2).Should(Succeed())
				Expect(sts.Spec.Template.Spec.ImagePullSecrets).To(ContainElement(corev1.LocalObjectReference{Name: secretName}))
			})

			It("reconciles", func() {
				resourceTests(rabbitmqCluster, clientSet, secretName)
			})
		})

		When("specified in the instance spec and config", func() {
			BeforeEach(func() {
				rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rabbitmq-two",
						Namespace: "rabbitmq-two",
					},
					Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
						Replicas:        1,
						ImagePullSecret: "rabbit-two-secret",
					},
				}

				Expect(client.Create(context.TODO(), rabbitmqCluster)).To(Succeed())
				waitForClusterCreation(rabbitmqCluster, client)
			})

			AfterEach(func() {
				Expect(client.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
			})

			It("does not create a new registry secret", func() {
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

			It("reconciles", func() {
				resourceTests(rabbitmqCluster, clientSet, "rabbit-two-secret")
			})
		})
	})
})

func waitForClusterCreation(rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster, client runtimeClient.Client) {
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
}

func resourceTests(rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster, clientset *kubernetes.Clientset, imagePullSecretName string) {
	By("creating the server conf configmap", func() {
		configMapName := rabbitmqCluster.ChildResourceName("server-conf")
		configMap, err := clientSet.CoreV1().ConfigMaps(rabbitmqCluster.Namespace).Get(configMapName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(configMap.Name).To(Equal(configMapName))
		Expect(configMap.OwnerReferences[0].Name).To(Equal(rabbitmqCluster.Name))
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

	By("creating a rabbitmq ingress service", func() {
		ingressServiceName := rabbitmqCluster.ChildResourceName("ingress")
		service, err := clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(ingressServiceName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(service.Name).To(Equal(ingressServiceName))
		Expect(service.OwnerReferences[0].Name).To(Equal(rabbitmqCluster.Name))
	})

	By("creating a rabbitmq headless service", func() {
		headlessServiceName := rabbitmqCluster.ChildResourceName("headless")
		service, err := clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(headlessServiceName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(service.Name).To(Equal(headlessServiceName))
		Expect(service.OwnerReferences[0].Name).To(Equal(rabbitmqCluster.Name))
	})

	By("creating a statefulset", func() {
		statefulSetName := rabbitmqCluster.ChildResourceName("server")
		sts, err := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(statefulSetName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(sts.Name).To(Equal(statefulSetName))
		Expect(sts.Spec.Template.Spec.ImagePullSecrets).To(ContainElement(corev1.LocalObjectReference{Name: imagePullSecretName}))
		Expect(sts.OwnerReferences[0].Name).To(Equal(rabbitmqCluster.Name))
	})

	By("creating a service account", func() {
		name := rabbitmqCluster.ChildResourceName("server")
		var serviceAccount *corev1.ServiceAccount
		Eventually(func() error {
			var err error
			serviceAccount, err = clientSet.CoreV1().ServiceAccounts(rabbitmqCluster.Namespace).Get(name, metav1.GetOptions{})
			return err
		}, 5).Should(Succeed())
		Expect(serviceAccount.Name).To(Equal(name))
	})

	By("creating a role", func() {
		name := rabbitmqCluster.ChildResourceName("endpoint-discovery")
		var role *rbacv1.Role
		Eventually(func() error {
			var err error
			role, err = clientSet.RbacV1().Roles(rabbitmqCluster.Namespace).Get(name, metav1.GetOptions{})
			return err
		}, 5).Should(Succeed())
		Expect(role.Name).To(Equal(name))
	})

	By("creating a role binding", func() {
		name := rabbitmqCluster.ChildResourceName("server")
		var roleBinding *rbacv1.RoleBinding
		Eventually(func() error {
			var err error
			roleBinding, err = clientSet.RbacV1().RoleBindings(rabbitmqCluster.Namespace).Get(name, metav1.GetOptions{})
			return err
		}, 5).Should(Succeed())
		Expect(roleBinding.Name).To(Equal(name))
	})
}
