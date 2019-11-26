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
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/controllers"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

const timeout = 3 * time.Second

var _ = Describe("RabbitmqclusterController", func() {

	var (
		rabbitmqCluster        *rabbitmqv1beta1.RabbitmqCluster
		operatorRegistrySecret *corev1.Secret
		secretName             = "rabbitmq-one-registry-access"
		managerConfig          resource.DefaultConfiguration
		scheme                 *runtime.Scheme
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(rabbitmqv1beta1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(defaultscheme.AddToScheme(scheme)).NotTo(HaveOccurred())
	})

	Context("Config updates", func() {
		BeforeEach(func() {
			managerConfig = resource.DefaultConfiguration{
				ImagePullSecret: "pivotal-rmq-registry-access",
			}

			startManager(scheme, managerConfig)

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

			Expect(client.Create(context.TODO(), rabbitmqCluster)).To(Succeed())
			waitForClusterCreation(rabbitmqCluster, client)

			stopManager()
			resourceRequirements := resource.ResourceRequirements{
				Limit: resource.ComputeResource{
					CPU: "2000m",
				},
				Request: resource.ComputeResource{
					CPU: "1000m",
				},
			}
			managerConfig := resource.DefaultConfiguration{
				ImagePullSecret:      "pivotal-rmq-registry-access",
				ServiceAnnotations:   map[string]string{"test-key": "test-value"},
				ResourceRequirements: resourceRequirements,
			}
			startManager(scheme, managerConfig)
			waitForClusterCreation(rabbitmqCluster, client)
		})

		AfterEach(func() {
			Expect(client.Delete(context.TODO(), operatorRegistrySecret)).To(Succeed())
			Expect(client.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
			stopManager()
		})

		It("does not impact existing instances", func() {
			ingressServiceName := rabbitmqCluster.ChildResourceName("ingress")
			service, err := clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(ingressServiceName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(service.Annotations).NotTo(HaveKeyWithValue("test-key", "test-value"))

			statefulSetName := rabbitmqCluster.ChildResourceName("server")
			sts, err := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(statefulSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(*sts.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu()).To(Equal(k8sresource.MustParse("500m")))
		})

		It("impacts new instances", func() {
			newRabbitmqCluster := &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-three",
					Namespace: "rabbitmq-three",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas:        1,
					ImagePullSecret: "rabbit-two-secret",
				},
			}

			Expect(client.Create(context.TODO(), newRabbitmqCluster)).To(Succeed())
			waitForClusterCreation(newRabbitmqCluster, client)
			ingressServiceName := newRabbitmqCluster.ChildResourceName("ingress")
			service, err := clientSet.CoreV1().Services(newRabbitmqCluster.Namespace).Get(ingressServiceName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(service.Annotations).To(HaveKeyWithValue("test-key", "test-value"))

			statefulSetName := newRabbitmqCluster.ChildResourceName("server")
			sts, err := clientSet.AppsV1().StatefulSets(newRabbitmqCluster.Namespace).Get(statefulSetName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(*sts.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu()).To(Equal(k8sresource.MustParse("2")))

			Expect(client.Delete(context.TODO(), newRabbitmqCluster)).To(Succeed())
		})
	})

	Context("Custom Resource updates", func() {
		BeforeEach(func() {
			managerConfig = resource.DefaultConfiguration{
				ImagePullSecret: "pivotal-rmq-registry-access",
			}

			startManager(scheme, managerConfig)

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

			Expect(client.Create(context.TODO(), rabbitmqCluster)).To(Succeed())
			waitForClusterCreation(rabbitmqCluster, client)
		})

		AfterEach(func() {
			Expect(client.Delete(context.TODO(), operatorRegistrySecret)).To(Succeed())
			Expect(client.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
			stopManager()
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
					service, err := clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(ingressServiceName, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					return service.Annotations
				}, 100).Should(HaveKeyWithValue("test-key", "test-value"))
			})

			When("the CPU requirements are updated", func() {
				var resourceRequirements corev1.ResourceRequirements
				expectedRequirements := corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: k8sresource.MustParse("1100m")},
					Limits:   corev1.ResourceList{corev1.ResourceCPU: k8sresource.MustParse("1200m")},
				}
				rabbitmqCluster.Spec.Resource.Request.CPU = "1100m"
				rabbitmqCluster.Spec.Resource.Limit.CPU = "1200m"
				Expect(client.Update(context.TODO(), rabbitmqCluster)).To(Succeed())

				Eventually(func() corev1.ResourceList {
					stsName := rabbitmqCluster.ChildResourceName("server")
					sts, err := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(stsName, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					resourceRequirements = sts.Spec.Template.Spec.Containers[0].Resources
					return resourceRequirements.Requests
				}, 100).Should(HaveKeyWithValue(corev1.ResourceCPU, expectedRequirements.Requests[corev1.ResourceCPU]))
				Expect(resourceRequirements.Limits).To(HaveKeyWithValue(corev1.ResourceCPU, expectedRequirements.Limits[corev1.ResourceCPU]))
			})
		})
	})

	Context("ImagePullSecret", func() {
		BeforeEach(func() {
			scheme := runtime.NewScheme()
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).NotTo(HaveOccurred())
			Expect(defaultscheme.AddToScheme(scheme)).NotTo(HaveOccurred())

			managerConfig = resource.DefaultConfiguration{
				ImagePullSecret: "pivotal-rmq-registry-access",
			}

			startManager(scheme, managerConfig)

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
			stopManager()
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
				_, err := clientSet.CoreV1().Secrets(rabbitmqCluster.Namespace).Get(secretName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				stsName := rabbitmqCluster.ChildResourceName("server")
				sts, err := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(stsName, metav1.GetOptions{})
				Expect(sts.Spec.Template.Spec.ImagePullSecrets).To(ContainElement(corev1.LocalObjectReference{Name: secretName}))
				Expect(err).NotTo(HaveOccurred())
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

func startManager(scheme *runtime.Scheme, config resource.DefaultConfiguration) {
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	client = mgr.GetClient()

	reconciler := &controllers.RabbitmqClusterReconciler{
		Client:                     client,
		Log:                        ctrl.Log.WithName("controllers").WithName("rabbitmqcluster"),
		Scheme:                     mgr.GetScheme(),
		Namespace:                  "pivotal-rabbitmq-system",
		ServiceType:                config.ServiceType,
		ServiceAnnotations:         config.ServiceAnnotations,
		Image:                      config.ImageReference,
		ImagePullSecret:            config.ImagePullSecret,
		PersistentStorage:          config.PersistentStorage,
		PersistentStorageClassName: config.PersistentStorageClassName,
		ResourceRequirements:       config.ResourceRequirements,
	}
	reconciler.SetupWithManager(mgr)

	stopMgr = make(chan struct{})
	mgrStopped = &sync.WaitGroup{}
	mgrStopped.Add(1)
	go func() {
		defer mgrStopped.Done()
		Expect(mgr.Start(stopMgr)).NotTo(HaveOccurred())
	}()
}

func stopManager() {
	close(stopMgr)
	mgrStopped.Wait()
}

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
}
