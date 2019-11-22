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
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
		managerConfig          config.Config
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(rabbitmqv1beta1.AddToScheme(scheme)).NotTo(HaveOccurred())
		Expect(defaultscheme.AddToScheme(scheme)).NotTo(HaveOccurred())

		managerConfig = config.Config{
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

	When("the imagePullSecret is specified only in config", func() {
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

		It("creating the registry secret", func() {
			Eventually(func() *corev1.Secret {
				secret, err := clientSet.CoreV1().Secrets(rabbitmqCluster.Namespace).Get(secretName, metav1.GetOptions{})
				if err != nil && apierrors.IsNotFound(err) {
					return nil
				}
				return secret
			}, 5).ShouldNot(BeNil())
			stsName := rabbitmqCluster.ChildResourceName("server")
			sts, err := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(stsName, metav1.GetOptions{})
			Expect(sts.Spec.Template.Spec.ImagePullSecrets).To(ContainElement(corev1.LocalObjectReference{Name: secretName}))
			Expect(err).NotTo(HaveOccurred())
			Expect(sts.Name).To(Equal(stsName))

		})

		It("reconciles", func() {
			resourceTests(rabbitmqCluster, clientSet)
		})
	})

	When("the imagePullSecret is specified in the instance spec (and config)", func() {
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
			stsName := rabbitmqCluster.ChildResourceName("server")
			var sts *appsv1.StatefulSet
			Eventually(func() *appsv1.StatefulSet {
				var err error
				sts, err = clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(stsName, metav1.GetOptions{})
				if err != nil && apierrors.IsNotFound(err) {
					return nil
				}
				return sts
			}, 5).ShouldNot(BeNil())
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

		It("reconciles", func() {
			resourceTests(rabbitmqCluster, clientSet)
		})
	})
})

func startManager(scheme *runtime.Scheme, config config.Config) {
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	client = mgr.GetClient()

	reconciler := &controllers.RabbitmqClusterReconciler{
		Client:                     client,
		Log:                        ctrl.Log.WithName("controllers").WithName("rabbitmqcluster"),
		Scheme:                     mgr.GetScheme(),
		Namespace:                  "pivotal-rabbitmq-system",
		ServiceType:                config.Service.Type,
		ServiceAnnotations:         config.Service.Annotations,
		Image:                      config.Image,
		ImagePullSecret:            config.ImagePullSecret,
		PersistentStorage:          config.Persistence.Storage,
		PersistentStorageClassName: config.Persistence.StorageClassName,
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

func resourceTests(rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster, clientset *kubernetes.Clientset) {
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
}
