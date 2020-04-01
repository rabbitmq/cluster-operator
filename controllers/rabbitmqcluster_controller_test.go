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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("RabbitmqclusterController", func() {

	var (
		rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster
	)

	Context("using minimal settings on the instance", func() {
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
			Eventually(func() bool {
				rmq := &rabbitmqv1beta1.RabbitmqCluster{}
				err := client.Get(context.TODO(), types.NamespacedName{Name: rabbitmqCluster.Name, Namespace: rabbitmqCluster.Namespace}, rmq)
				return apierrors.IsNotFound(err)
			}, 5).Should(BeTrue())
		})

		It("works", func() {
			By("creating a statefulset with default configurations", func() {
				statefulSetName := rabbitmqCluster.ChildResourceName("server")
				sts := statefulSet(rabbitmqCluster)
				Expect(sts.Name).To(Equal(statefulSetName))

				Expect(sts.Spec.Template.Spec.ImagePullSecrets).To(BeEmpty())

				Expect(len(sts.Spec.VolumeClaimTemplates)).To(Equal(1))
				Expect(sts.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(BeNil())
			})

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
				Expect(secret.OwnerReferences[0].Name).To(Equal(rabbitmqCluster.Name))
			})

			By("creating an erlang cookie secret", func() {
				secretName := rabbitmqCluster.ChildResourceName("erlang-cookie")
				secret, err := clientSet.CoreV1().Secrets(rabbitmqCluster.Namespace).Get(secretName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Name).To(Equal(secretName))
				Expect(secret.OwnerReferences[0].Name).To(Equal(rabbitmqCluster.Name))
			})

			By("creating a rabbitmq ingress service", func() {
				svc := service(rabbitmqCluster, "ingress")
				Expect(svc.Name).To(Equal(rabbitmqCluster.ChildResourceName("ingress")))
				Expect(svc.OwnerReferences[0].Name).To(Equal(rabbitmqCluster.Name))
				Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
			})

			By("creating a rabbitmq headless service", func() {
				svc := service(rabbitmqCluster, "headless")
				Expect(svc.Name).To(Equal(rabbitmqCluster.ChildResourceName("headless")))
				Expect(svc.OwnerReferences[0].Name).To(Equal(rabbitmqCluster.Name))
			})

			By("creating a service account", func() {
				serviceAccountName := rabbitmqCluster.ChildResourceName("server")
				serviceAccount, err := clientSet.CoreV1().ServiceAccounts(rabbitmqCluster.Namespace).Get(serviceAccountName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(serviceAccount.Name).To(Equal(serviceAccountName))
				Expect(serviceAccount.OwnerReferences[0].Name).To(Equal(rabbitmqCluster.Name))
			})

			By("creating a role", func() {
				roleName := rabbitmqCluster.ChildResourceName("endpoint-discovery")
				role, err := clientSet.RbacV1().Roles(rabbitmqCluster.Namespace).Get(roleName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(role.Name).To(Equal(roleName))
				Expect(role.OwnerReferences[0].Name).To(Equal(rabbitmqCluster.Name))
			})

			By("creating a role binding", func() {
				roleBindingName := rabbitmqCluster.ChildResourceName("server")
				roleBinding, err := clientSet.RbacV1().RoleBindings(rabbitmqCluster.Namespace).Get(roleBindingName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(roleBinding.Name).To(Equal(roleBindingName))
				Expect(roleBinding.OwnerReferences[0].Name).To(Equal(rabbitmqCluster.Name))
			})
			By("recording SuccessfullCreate events for all child resources", func() {
				allEventMsgs := aggregateEventMsgs(rabbitmqCluster, "SuccessfulCreate")
				Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("created resource %s of Type *v1.StatefulSet", rabbitmqCluster.ChildResourceName("server"))))
				Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("created resource %s of Type *v1.Service", rabbitmqCluster.ChildResourceName("ingress"))))
				Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("created resource %s of Type *v1.Service", rabbitmqCluster.ChildResourceName("headless"))))
				Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("created resource %s of Type *v1.ConfigMap", rabbitmqCluster.ChildResourceName("server-conf"))))
				Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("created resource %s of Type *v1.Secret", rabbitmqCluster.ChildResourceName("erlang-cookie"))))
				Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("created resource %s of Type *v1.Secret", rabbitmqCluster.ChildResourceName("admin"))))
				Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("created resource %s of Type *v1.ServiceAccount", rabbitmqCluster.ChildResourceName("server"))))
				Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("created resource %s of Type *v1.Role", rabbitmqCluster.ChildResourceName("endpoint-discovery"))))
				Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("created resource %s of Type *v1.RoleBinding", rabbitmqCluster.ChildResourceName("server"))))
			})

			By("adding the deletion finalizer", func() {
				rmq := &rabbitmqv1beta1.RabbitmqCluster{}
				Eventually(func() string {
					err := client.Get(context.TODO(), types.NamespacedName{Name: rabbitmqCluster.Name, Namespace: rabbitmqCluster.Namespace}, rmq)
					if err != nil {
						return ""
					}
					if len(rmq.Finalizers) > 0 {
						return rmq.Finalizers[0]
					}

					return ""
				}, 5).Should(Equal("deletion.finalizers.rabbitmq"))
			})
		})
	})

	Context("Annotations set on the instance", func() {
		BeforeEach(func() {
			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-annotations",
					Namespace: "rabbitmq-annotations",
					Annotations: map[string]string{
						"my-annotation": "this-annotation",
					},
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

		It("adds annotations to child resources", func() {
			headlessSvc := service(rabbitmqCluster, "headless")
			Expect(headlessSvc.Annotations).Should(HaveKeyWithValue("my-annotation", "this-annotation"))

			sts := statefulSet(rabbitmqCluster)
			Expect(sts.Annotations).Should(HaveKeyWithValue("my-annotation", "this-annotation"))
		})

	})

	Context("ImagePullSecret configure on the instance", func() {
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

		It("configures the imagePullSecret on sts correctly", func() {
			By("using the instance spec secret", func() {
				sts := statefulSet(rabbitmqCluster)
				Expect(sts.Spec.Template.Spec.ImagePullSecrets).Should(ContainElement(corev1.LocalObjectReference{Name: "rabbit-two-secret"}))
			})
		})
	})

	Context("Affinity configurations", func() {
		var affinity = &corev1.Affinity{
			PodAffinity: &corev1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"affinity-label": "anti-affinity",
							},
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
		}
		BeforeEach(func() {
			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-affinity",
					Namespace: "rabbitmq-affinity",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas:        1,
					Affinity:        affinity,
					ImagePullSecret: "rabbit-two-secret",
				},
			}
			Expect(client.Create(context.TODO(), rabbitmqCluster)).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(client.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
		})

		It("adds the affinity rules to pod spec", func() {
			sts := statefulSet(rabbitmqCluster)
			podSpecAffinity := sts.Spec.Template.Spec.Affinity
			Expect(podSpecAffinity).To(Equal(affinity))
		})
	})

	Context("Ingress service configurations", func() {
		AfterEach(func() {
			Expect(client.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
			Expect(clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Delete(rabbitmqCluster.ChildResourceName("ingress"), &metav1.DeleteOptions{}))
		})

		It("creates the service type and annotations as configured in instance spec", func() {
			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbit-service-2",
					Namespace: "rabbit-service-2",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas:        1,
					ImagePullSecret: "rabbit-service-secret",
				},
			}
			rabbitmqCluster.Spec.Service.Type = "LoadBalancer"
			rabbitmqCluster.Spec.Service.Annotations = map[string]string{"annotations": "cr-annotation"}

			Expect(client.Create(context.TODO(), rabbitmqCluster)).NotTo(HaveOccurred())

			ingressSvc := service(rabbitmqCluster, "ingress")
			Expect(ingressSvc.Spec.Type).Should(Equal(corev1.ServiceTypeLoadBalancer))
			Expect(ingressSvc.Annotations).Should(HaveKeyWithValue("annotations", "cr-annotation"))
		})
	})

	Context("Resource requirements configurations", func() {
		AfterEach(func() {
			Expect(client.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
		})

		It("uses resource requirements from instance spec when provided", func() {
			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbit-resource-2",
					Namespace: "rabbit-resource-2",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas:        1,
					ImagePullSecret: "rabbit-resource-secret",
				},
			}
			rabbitmqCluster.Spec.Resources = &corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]k8sresource.Quantity{
					corev1.ResourceMemory: k8sresource.MustParse("4Gi"),
				},
				Requests: map[corev1.ResourceName]k8sresource.Quantity{
					corev1.ResourceMemory: k8sresource.MustParse("4Gi"),
				},
			}

			Expect(client.Create(context.TODO(), rabbitmqCluster)).NotTo(HaveOccurred())

			sts := statefulSet(rabbitmqCluster)

			actualResources := sts.Spec.Template.Spec.Containers[0].Resources
			expectedResources := corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]k8sresource.Quantity{
					corev1.ResourceMemory: k8sresource.MustParse("4Gi"),
				},
				Requests: map[corev1.ResourceName]k8sresource.Quantity{
					corev1.ResourceMemory: k8sresource.MustParse("4Gi"),
				},
			}

			Expect(actualResources).To(Equal(expectedResources))

		})
	})

	Context("Persistence configurations", func() {
		AfterEach(func() {
			Expect(client.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
		})

		It("creates the RabbitmqCluster with the specified storage from instance spec", func() {
			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbit-persistence-1",
					Namespace: "rabbit-persistence-1",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas:        1,
					ImagePullSecret: "rabbit-resource-secret",
				},
			}
			storageClassName := "my-storage-class"
			rabbitmqCluster.Spec.Persistence.StorageClassName = &storageClassName
			storage := k8sresource.MustParse("100Gi")
			rabbitmqCluster.Spec.Persistence.Storage = &storage
			Expect(client.Create(context.TODO(), rabbitmqCluster)).NotTo(HaveOccurred())

			sts := statefulSet(rabbitmqCluster)

			Expect(len(sts.Spec.VolumeClaimTemplates)).To(Equal(1))
			Expect(*sts.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(Equal("my-storage-class"))
			actualStorageCapacity := sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]
			Expect(actualStorageCapacity).To(Equal(k8sresource.MustParse("100Gi")))
		})
	})

	Context("Custom Resource updates", func() {
		var (
			rabbitmqCluster    *rabbitmqv1beta1.RabbitmqCluster
			ingressServiceName string
			statefulSetName    string
		)
		BeforeEach(func() {
			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-cr-update",
					Namespace: "rabbitmq-cr-update",
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
			Expect(client.Get(
				context.TODO(),
				types.NamespacedName{Name: rabbitmqCluster.Name, Namespace: rabbitmqCluster.Namespace},
				rabbitmqCluster,
			)).To(Succeed())
		})

		AfterEach(func() {
			Expect(client.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
			waitForClusterDeletion(rabbitmqCluster, client)
		})

		It("the service annotations are updated", func() {
			rabbitmqCluster.Spec.Service.Annotations = map[string]string{"test-key": "test-value"}
			Expect(client.Update(context.TODO(), rabbitmqCluster)).To(Succeed())
			Eventually(func() map[string]string {
				ingressServiceName := rabbitmqCluster.ChildResourceName("ingress")
				service, _ := clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(ingressServiceName, metav1.GetOptions{})
				return service.Annotations
			}, 1).Should(HaveKeyWithValue("test-key", "test-value"))

			// verify that SuccessfulUpdate event is recorded for the ingress service
			Expect(aggregateEventMsgs(rabbitmqCluster, "SuccessfulUpdate")).To(
				ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.Service", rabbitmqCluster.ChildResourceName("ingress"))))
		})

		It("the CPU and memory requirements are updated", func() {
			var resourceRequirements corev1.ResourceRequirements
			expectedRequirements := &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    k8sresource.MustParse("1100m"),
					corev1.ResourceMemory: k8sresource.MustParse("5Gi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    k8sresource.MustParse("1200m"),
					corev1.ResourceMemory: k8sresource.MustParse("6Gi"),
				},
			}
			rabbitmqCluster.Spec.Resources = expectedRequirements
			Expect(client.Update(context.TODO(), rabbitmqCluster)).To(Succeed())

			Eventually(func() corev1.ResourceList {
				stsName := rabbitmqCluster.ChildResourceName("server")
				sts, err := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(stsName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				resourceRequirements = sts.Spec.Template.Spec.Containers[0].Resources
				return resourceRequirements.Requests
			}, 1).Should(HaveKeyWithValue(corev1.ResourceCPU, expectedRequirements.Requests[corev1.ResourceCPU]))
			Expect(resourceRequirements.Limits).To(HaveKeyWithValue(corev1.ResourceCPU, expectedRequirements.Limits[corev1.ResourceCPU]))

			Expect(resourceRequirements.Requests).To(HaveKeyWithValue(corev1.ResourceMemory, expectedRequirements.Requests[corev1.ResourceMemory]))
			Expect(resourceRequirements.Limits).To(HaveKeyWithValue(corev1.ResourceMemory, expectedRequirements.Limits[corev1.ResourceMemory]))

			// verify that SuccessfulUpdate event is recorded for the StatefulSet
			Expect(aggregateEventMsgs(rabbitmqCluster, "SuccessfulUpdate")).To(
				ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.StatefulSet", rabbitmqCluster.ChildResourceName("server"))))
		})

		It("the rabbitmq image is updated", func() {
			rabbitmqCluster.Spec.Image = "rabbitmq:3.8.0"
			Expect(client.Update(context.TODO(), rabbitmqCluster)).To(Succeed())

			Eventually(func() string {
				stsName := rabbitmqCluster.ChildResourceName("server")
				sts, _ := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(stsName, metav1.GetOptions{})
				return sts.Spec.Template.Spec.Containers[0].Image
			}, 1).Should(Equal("rabbitmq:3.8.0"))
		})

		It("the rabbitmq ImagePullSecret is updated", func() {
			rabbitmqCluster.Spec.ImagePullSecret = "my-new-secret"
			Expect(client.Update(context.TODO(), rabbitmqCluster)).To(Succeed())

			Eventually(func() []corev1.LocalObjectReference {
				stsName := rabbitmqCluster.ChildResourceName("server")
				sts, _ := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(stsName, metav1.GetOptions{})
				Expect(len(sts.Spec.Template.Spec.ImagePullSecrets)).To(Equal(1))
				return sts.Spec.Template.Spec.ImagePullSecrets
			}, 1).Should(ConsistOf(corev1.LocalObjectReference{Name: "my-new-secret"}))
		})

		It("labels are updated", func() {
			rabbitmqCluster.Labels = make(map[string]string)
			rabbitmqCluster.Labels["foo"] = "bar"

			Expect(client.Update(context.TODO(), rabbitmqCluster)).To(Succeed())

			Eventually(func() map[string]string {
				service, err := clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(ingressServiceName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				return service.Labels
			}, 1).Should(HaveKeyWithValue("foo", "bar"))
			var sts *appsv1.StatefulSet
			Eventually(func() map[string]string {
				sts, _ = clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(statefulSetName, metav1.GetOptions{})
				return sts.Labels
			}, 1).Should(HaveKeyWithValue("foo", "bar"))
		})

		It("instance annotations are updated", func() {
			rabbitmqCluster.Annotations = make(map[string]string)
			rabbitmqCluster.Annotations["anno-key"] = "anno-value"
			Expect(client.Update(context.TODO(), rabbitmqCluster)).To(Succeed())

			Eventually(func() map[string]string {
				service, err := clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(rabbitmqCluster.ChildResourceName("headless"), metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				return service.Annotations
			}, 1).Should(HaveKeyWithValue("anno-key", "anno-value"))
			var sts *appsv1.StatefulSet
			Eventually(func() map[string]string {
				sts, _ = clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(statefulSetName, metav1.GetOptions{})
				return sts.Annotations
			}, 1).Should(HaveKeyWithValue("anno-key", "anno-value"))

			// verify that SuccessfulUpdate events are recorded for all child resources
			allEventMsgs := aggregateEventMsgs(rabbitmqCluster, "SuccessfulUpdate")
			Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.StatefulSet", rabbitmqCluster.ChildResourceName("server"))))
			Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.Service", rabbitmqCluster.ChildResourceName("ingress"))))
			Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.Service", rabbitmqCluster.ChildResourceName("headless"))))
			Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.ConfigMap", rabbitmqCluster.ChildResourceName("server-conf"))))
			Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.Secret", rabbitmqCluster.ChildResourceName("erlang-cookie"))))
			Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.Secret", rabbitmqCluster.ChildResourceName("admin"))))
			Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.ServiceAccount", rabbitmqCluster.ChildResourceName("server"))))
			Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.Role", rabbitmqCluster.ChildResourceName("endpoint-discovery"))))
			Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.RoleBinding", rabbitmqCluster.ChildResourceName("server"))))
		})

		It("service type is updated", func() {
			rabbitmqCluster.Spec.Service.Type = "NodePort"
			Expect(client.Update(context.TODO(), rabbitmqCluster)).To(Succeed())
			Eventually(func() string {
				service, err := clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(rabbitmqCluster.ChildResourceName("ingress"), metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				return string(service.Spec.Type)
			}, 1).Should(Equal("NodePort"))
		})

		It("affinity rules are updated", func() {
			affinity := &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "foo",
										Operator: "Exists",
										Values:   nil,
									},
								},
							},
						},
					},
				},
			}

			rabbitmqCluster.Spec.Affinity = affinity
			Expect(client.Update(context.TODO(), rabbitmqCluster)).To(Succeed())
			waitForClusterCreation(rabbitmqCluster, client)
			Eventually(func() *corev1.Affinity {
				sts, _ := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(statefulSetName, metav1.GetOptions{})
				return sts.Spec.Template.Spec.Affinity
			}, 1).Should(Equal(affinity))

			Expect(client.Get(
				context.TODO(),
				types.NamespacedName{Name: rabbitmqCluster.Name, Namespace: rabbitmqCluster.Namespace},
				rabbitmqCluster,
			)).To(Succeed())
			affinity = nil
			rabbitmqCluster.Spec.Affinity = affinity
			Expect(client.Update(context.TODO(), rabbitmqCluster)).To(Succeed())
			Eventually(func() *corev1.Affinity {
				sts, _ := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(statefulSetName, metav1.GetOptions{})
				return sts.Spec.Template.Spec.Affinity
			}, 1).Should(BeNil())
		})
	})

	Context("Recreate child resources after deletion", func() {
		var (
			ingressServiceName  string
			headlessServiceName string
			stsName             string
			configMapName       string
			namespace           string
		)
		BeforeEach(func() {
			namespace = "rabbitmq-delete"
			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-delete",
					Namespace: namespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas:        1,
					ImagePullSecret: "rabbit-two-secret",
				},
			}
			ingressServiceName = rabbitmqCluster.ChildResourceName("ingress")
			headlessServiceName = rabbitmqCluster.ChildResourceName("headless")
			stsName = rabbitmqCluster.ChildResourceName("server")
			configMapName = rabbitmqCluster.ChildResourceName("server-conf")

			Expect(client.Create(context.TODO(), rabbitmqCluster)).To(Succeed())
			time.Sleep(500 * time.Millisecond)
		})

		AfterEach(func() {
			Expect(client.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
		})

		It("recreates child resources after deletion", func() {
			oldConfMap, err := clientSet.CoreV1().ConfigMaps(namespace).Get(configMapName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			oldIngressSvc := service(rabbitmqCluster, "ingress")

			oldHeadlessSvc := service(rabbitmqCluster, "headless")

			oldSts := statefulSet(rabbitmqCluster)

			Expect(clientSet.AppsV1().StatefulSets(namespace).Delete(stsName, &metav1.DeleteOptions{})).NotTo(HaveOccurred())
			Expect(clientSet.CoreV1().ConfigMaps(namespace).Delete(configMapName, &metav1.DeleteOptions{})).NotTo(HaveOccurred())
			Expect(clientSet.CoreV1().Services(namespace).Delete(ingressServiceName, &metav1.DeleteOptions{})).NotTo(HaveOccurred())
			Expect(clientSet.CoreV1().Services(namespace).Delete(headlessServiceName, &metav1.DeleteOptions{})).NotTo(HaveOccurred())

			Eventually(func() bool {
				sts, err := clientSet.AppsV1().StatefulSets(namespace).Get(stsName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return string(sts.UID) != string(oldSts.UID)
			}, 5).Should(BeTrue())

			Eventually(func() bool {
				ingressSvc, err := clientSet.CoreV1().Services(namespace).Get(ingressServiceName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return string(ingressSvc.UID) != string(oldIngressSvc.UID)
			}, 5).Should(BeTrue())

			Eventually(func() bool {
				headlessSvc, err := clientSet.CoreV1().Services(namespace).Get(headlessServiceName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return string(headlessSvc.UID) != string(oldHeadlessSvc.UID)
			}, 5).Should(Not(Equal(oldHeadlessSvc.UID)))

			Eventually(func() bool {
				configMap, err := clientSet.CoreV1().ConfigMaps(namespace).Get(configMapName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return string(configMap.UID) != string(oldConfMap.UID)
			}, 5).Should(Not(Equal(oldConfMap.UID)))

		})
	})

})

// aggregateEventMsgs - helper function to aggregate all event messages for a given rabbitmqcluster
// and filters on a specific event reason string
func aggregateEventMsgs(rabbit *rabbitmqv1beta1.RabbitmqCluster, reason string) string {
	events, err := clientSet.CoreV1().Events(rabbit.Namespace).List(metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.namespace=%s,reason=%s", rabbit.Name, rabbit.Namespace, reason),
	})
	ExpectWithOffset(1, err).To(Succeed())
	var msgs string
	for _, e := range events.Items {
		msgs = msgs + e.Message + " "
	}
	return msgs
}

func statefulSet(rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) *appsv1.StatefulSet {
	stsName := rabbitmqCluster.ChildResourceName("server")
	var sts *appsv1.StatefulSet
	EventuallyWithOffset(1, func() error {
		var err error
		sts, err = clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(stsName, metav1.GetOptions{})
		return err
	}, 5).Should(Succeed())
	return sts
}

func service(rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster, svcName string) *corev1.Service {
	serviceName := rabbitmqCluster.ChildResourceName(svcName)
	var svc *corev1.Service
	EventuallyWithOffset(1, func() error {
		var err error
		svc, err = clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(serviceName, metav1.GetOptions{})
		return err
	}, 2).Should(Succeed())
	return svc
}

func waitForClusterCreation(rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster, client runtimeClient.Client) {
	EventuallyWithOffset(1, func() string {
		rabbitmqClusterCreated := rabbitmqv1beta1.RabbitmqCluster{}
		err := client.Get(
			context.TODO(),
			types.NamespacedName{Name: rabbitmqCluster.Name, Namespace: rabbitmqCluster.Namespace},
			&rabbitmqClusterCreated,
		)
		if err != nil {
			return fmt.Sprintf("%v+", err)
		}

		if len(rabbitmqClusterCreated.Status.Conditions) == 0 {
			return fmt.Sprintf("not ready")
		}

		return "ready"

	}, 5, 1).Should(Equal("ready"))

}

func waitForClusterDeletion(rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster, client runtimeClient.Client) {
	EventuallyWithOffset(1, func() bool {
		rabbitmqClusterCreated := rabbitmqv1beta1.RabbitmqCluster{}
		err := client.Get(
			context.TODO(),
			types.NamespacedName{Name: rabbitmqCluster.Name, Namespace: rabbitmqCluster.Namespace},
			&rabbitmqClusterCreated,
		)
		return apierrors.IsNotFound(err)
	}, 2, 1).Should(BeTrue())

}
