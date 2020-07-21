/*
RabbitMQ Cluster Operator

Copyright 2020 VMware, Inc. All Rights Reserved.

This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package controllers_test

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/util/intstr"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ClusterCreationTimeout = 5 * time.Second
	ClusterDeletionTimeout = 5 * time.Second
)

var _ = Describe("RabbitmqclusterController", func() {

	var (
		rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster
		one             int32 = 1
		updateWithRetry       = func(cr *rabbitmqv1beta1.RabbitmqCluster, mutateFn func(r *rabbitmqv1beta1.RabbitmqCluster)) error {
			return retry.RetryOnConflict(retry.DefaultRetry, func() error {
				objKey, err := runtimeClient.ObjectKeyFromObject(cr)
				if err != nil {
					return err
				}

				if err := client.Get(context.TODO(), objKey, cr); err != nil {
					return err
				}

				mutateFn(cr)

				return client.Update(context.TODO(), cr)
			})
		}
	)

	Context("using minimal settings on the instance", func() {
		BeforeEach(func() {
			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-one",
					Namespace: "rabbitmq-one",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: &one,
				},
			}

			Expect(client.Create(context.TODO(), rabbitmqCluster)).To(Succeed())
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

			By("creating a rabbitmq client service", func() {
				svc := service(rabbitmqCluster, "client")
				Expect(svc.Name).To(Equal(rabbitmqCluster.ChildResourceName("client")))
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
				Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("created resource %s of Type *v1.Service", rabbitmqCluster.ChildResourceName("client"))))
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
				}, 5).Should(Equal("deletion.finalizers.rabbitmqclusters.rabbitmq.com"))
			})

			By("setting the admin secret details in the custom resource status", func() {
				rmq := &rabbitmqv1beta1.RabbitmqCluster{}
				secretRef := &rabbitmqv1beta1.RabbitmqClusterSecretReference{}
				Eventually(func() *rabbitmqv1beta1.RabbitmqClusterSecretReference {
					err := client.Get(context.TODO(), types.NamespacedName{Name: rabbitmqCluster.Name, Namespace: rabbitmqCluster.Namespace}, rmq)
					if err != nil {
						return nil
					}

					if rmq.Status.Admin != nil && rmq.Status.Admin.SecretReference != nil {
						secretRef = rmq.Status.Admin.SecretReference
						return secretRef
					}

					return nil
				}, 5).ShouldNot(BeNil())

				Expect(secretRef.Name).To(Equal(rmq.ChildResourceName(resource.AdminSecretName)))
				Expect(secretRef.Namespace).To(Equal(rmq.Namespace))
				Expect(secretRef.Keys["username"]).To(Equal("username"))
				Expect(secretRef.Keys["password"]).To(Equal("password"))
			})

			By("setting the client service details in the custom resource status", func() {
				rmq := &rabbitmqv1beta1.RabbitmqCluster{}
				serviceRef := &rabbitmqv1beta1.RabbitmqClusterServiceReference{}
				Eventually(func() *rabbitmqv1beta1.RabbitmqClusterServiceReference {
					err := client.Get(context.TODO(), types.NamespacedName{Name: rabbitmqCluster.Name, Namespace: rabbitmqCluster.Namespace}, rmq)
					if err != nil {
						return nil
					}

					if rmq.Status.Admin != nil && rmq.Status.Admin.ServiceReference != nil {
						serviceRef = rmq.Status.Admin.ServiceReference
						return serviceRef
					}

					return nil
				}, 5).ShouldNot(BeNil())

				Expect(serviceRef.Name).To(Equal(rmq.ChildResourceName("client")))
				Expect(serviceRef.Namespace).To(Equal(rmq.Namespace))
			})
		})
	})

	Context("Mutual TLS with single secret", func() {
		AfterEach(func() {
			Expect(client.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
			Eventually(func() bool {
				rmq := &rabbitmqv1beta1.RabbitmqCluster{}
				err := client.Get(context.TODO(), types.NamespacedName{Name: rabbitmqCluster.Name, Namespace: rabbitmqCluster.Namespace}, rmq)
				return apierrors.IsNotFound(err)
			}, 5).Should(BeTrue())
		})
		It("Deploys successfully", func() {
			tlsSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-secret",
					Namespace: "rabbitmq-mutual-tls",
				},
				StringData: map[string]string{
					"tls.crt": "this is a tls cert",
					"tls.key": "this is a tls key",
					"caCERT":  "certificate",
				},
			}
			_, err := clientSet.CoreV1().Secrets("rabbitmq-mutual-tls").Create(&tlsSecret)
			if !apierrors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-mutual-tls",
					Namespace: "rabbitmq-mutual-tls",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: &one,
					TLS: rabbitmqv1beta1.TLSSpec{
						SecretName:   "tls-secret",
						CaSecretName: "tls-secret",
						CaCertName:   "caCERT",
					},
				},
			}

			Expect(client.Create(context.TODO(), rabbitmqCluster)).To(Succeed())
			waitForClusterCreation(rabbitmqCluster, client)
			sts, err := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(rabbitmqCluster.ChildResourceName("server"), metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			volumeMount := corev1.VolumeMount{
				Name:      "rabbitmq-tls",
				MountPath: "/etc/rabbitmq-tls/caCERT",
				SubPath:   "caCERT",
				ReadOnly:  true,
			}
			Expect(sts.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElement(volumeMount))
		})

		It("Does not deploy if the cert name does not match the contents of the secret", func() {
			tlsSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-secret-missing",
					Namespace: "rabbitmq-mutual-tls",
				},
				StringData: map[string]string{
					"tls.crt": "this is a tls cert",
					"tls.key": "this is a tls key",
					"ca.c":    "certificate",
				},
			}
			_, err := clientSet.CoreV1().Secrets("rabbitmq-mutual-tls").Create(&tlsSecret)
			if !apierrors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-mutual-tls-missing",
					Namespace: "rabbitmq-mutual-tls",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: &one,
					TLS: rabbitmqv1beta1.TLSSpec{
						SecretName:   "tls-secret-missing",
						CaSecretName: "tls-secret-missing",
						CaCertName:   "ca.crt",
					},
				},
			}

			Expect(client.Create(context.TODO(), rabbitmqCluster)).To(Succeed())

			tlsEventTimeout := 5 * time.Second
			tlsRetry := 1 * time.Second
			Eventually(func() string {
				return aggregateEventMsgs(rabbitmqCluster, "TLSError")
			}, tlsEventTimeout, tlsRetry).Should(
				ContainSubstring("The TLS secret tls-secret-missing in namespace rabbitmq-mutual-tls must have the field ca.crt"))
		})
	})

	Context("Mutual TLS with a seperate CA certificate secret", func() {
		AfterEach(func() {
			Expect(client.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
			Eventually(func() bool {
				rmq := &rabbitmqv1beta1.RabbitmqCluster{}
				err := client.Get(context.TODO(), types.NamespacedName{Name: rabbitmqCluster.Name, Namespace: rabbitmqCluster.Namespace}, rmq)
				return apierrors.IsNotFound(err)
			}, 5).Should(BeTrue())
		})
		It("Does not deploy the RabbitmqCluster, and retries every 10 seconds", func() {
			tlsSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-secret",
					Namespace: "rabbitmq-mutual-tls-ca",
				},
				StringData: map[string]string{
					"tls.crt": "this is a tls cert",
					"tls.key": "this is a tls key",
				},
			}
			_, err := clientSet.CoreV1().Secrets("rabbitmq-mutual-tls-ca").Create(&tlsSecret)
			if !apierrors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}
			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-tls-secret-does-not-exist",
					Namespace: "rabbitmq-mutual-tls-ca",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: &one,
					TLS: rabbitmqv1beta1.TLSSpec{
						SecretName:   "tls-secret",
						CaSecretName: "ca-cert-secret",
						CaCertName:   "ca.crt",
					},
				},
			}
			Expect(client.Create(context.TODO(), rabbitmqCluster)).To(Succeed())

			tlsEventTimeout := 15 * time.Second
			tlsRetry := 1 * time.Second
			Eventually(func() string {
				return aggregateEventMsgs(rabbitmqCluster, "TLSError")
			}, tlsEventTimeout, tlsRetry).Should(ContainSubstring("Failed to get CA certificate secret"))
			_, err = clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(rabbitmqCluster.ChildResourceName("server"), metav1.GetOptions{})
			Expect(err).To(HaveOccurred())

			// create missing secret
			caCertSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ca-cert-secret",
					Namespace: "rabbitmq-mutual-tls-ca",
				},
				StringData: map[string]string{
					"ca.crt": "this is a ca cert",
				},
			}
			_, err = clientSet.CoreV1().Secrets("rabbitmq-mutual-tls-ca").Create(&caCertSecret)
			Expect(err).NotTo(HaveOccurred())

			waitForClusterCreation(rabbitmqCluster, client)
			statefulSet(rabbitmqCluster)
		})
		It("Does not deploy if the cert name does not match the contents of the secret", func() {
			tlsSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-secret",
					Namespace: "rabbitmq-mutual-tls-ca-missing",
				},
				StringData: map[string]string{
					"tls.crt": "this is a tls cert",
					"tls.key": "this is a tls key",
				},
			}
			_, err := clientSet.CoreV1().Secrets("rabbitmq-mutual-tls-ca-missing").Create(&tlsSecret)
			if !apierrors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			caCertSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ca-cert-secret",
					Namespace: "rabbitmq-mutual-tls-ca-missing",
				},
				StringData: map[string]string{
					"cacrt": "this is a ca cert",
				},
			}
			_, err = clientSet.CoreV1().Secrets("rabbitmq-mutual-tls-ca-missing").Create(&caCertSecret)
			if !apierrors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-mutual-tls-missing",
					Namespace: "rabbitmq-mutual-tls-ca-missing",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: &one,
					TLS: rabbitmqv1beta1.TLSSpec{
						SecretName:   "tls-secret",
						CaSecretName: "ca-cert-secret",
						CaCertName:   "ca.crt",
					},
				},
			}

			Expect(client.Create(context.TODO(), rabbitmqCluster)).To(Succeed())

			tlsEventTimeout := 5 * time.Second
			tlsRetry := 1 * time.Second
			Eventually(func() string {
				return aggregateEventMsgs(rabbitmqCluster, "TLSError")
			}, tlsEventTimeout, tlsRetry).Should(
				ContainSubstring("The TLS secret tls-secret in namespace rabbitmq-mutual-tls-ca-missing must have the field ca.crt"))
		})
	})

	Context("TLS set on the instance", func() {
		AfterEach(func() {
			Expect(client.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
			Eventually(func() bool {
				rmq := &rabbitmqv1beta1.RabbitmqCluster{}
				err := client.Get(context.TODO(), types.NamespacedName{Name: rabbitmqCluster.Name, Namespace: rabbitmqCluster.Namespace}, rmq)
				return apierrors.IsNotFound(err)
			}, 5).Should(BeTrue())
		})
		BeforeEach(func() {
			tlsSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-secret",
					Namespace: "rabbitmq-tls",
				},
				StringData: map[string]string{
					"tls.crt": "this is a tls cert",
					"tls.key": "this is a tls key",
				},
			}
			_, err := clientSet.CoreV1().Secrets("rabbitmq-tls").Create(&tlsSecret)
			if !apierrors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-tls",
					Namespace: "rabbitmq-tls",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: &one,
					TLS: rabbitmqv1beta1.TLSSpec{
						SecretName: "tls-secret",
					},
				},
			}
		})

		It("Deploys successfully", func() {
			Expect(client.Create(context.TODO(), rabbitmqCluster)).To(Succeed())
			waitForClusterCreation(rabbitmqCluster, client)
		})

		When("the TLS secret does not have the expected keys - tls.crt, or tls.key", func() {
			BeforeEach(func() {
				malformedSecret := corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rabbitmq-tls-malformed",
						Namespace: "rabbitmq-tls-namespace",
					},
					StringData: map[string]string{
						"somekey": "someval",
						"tls.key": "this is a tls key",
					},
				}
				_, err := clientSet.CoreV1().Secrets("rabbitmq-tls-namespace").Create(&malformedSecret)
				if !apierrors.IsAlreadyExists(err) {
					Expect(err).NotTo(HaveOccurred())
				}

				rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rabbitmq-tls-malformed",
						Namespace: "rabbitmq-tls-namespace",
					},
					Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
						Replicas: &one,
						TLS: rabbitmqv1beta1.TLSSpec{
							SecretName: "rabbitmq-tls-malformed",
						},
					},
				}
			})

			It("fails to deploy the RabbitmqCluster", func() {
				Expect(client.Create(context.TODO(), rabbitmqCluster)).To(Succeed())

				tlsEventTimeout := 5 * time.Second
				tlsRetry := 1 * time.Second
				Eventually(func() string {
					return aggregateEventMsgs(rabbitmqCluster, "TLSError")
				}, tlsEventTimeout, tlsRetry).Should(
					ContainSubstring("The TLS secret rabbitmq-tls-malformed in namespace rabbitmq-tls-namespace must have the fields tls.crt and tls.key"))
			})
		})

		When("the TLS secret does not exist", func() {
			It("fails to deploy the RabbitmqCluster until the secret is detected", func() {
				rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rabbitmq-tls-secret-does-not-exist",
						Namespace: "rabbitmq-namespace",
					},
					Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
						Replicas: &one,
						TLS: rabbitmqv1beta1.TLSSpec{
							SecretName: "tls-secret-does-not-exist",
						},
					},
				}
				Expect(client.Create(context.TODO(), rabbitmqCluster)).To(Succeed())

				tlsEventTimeout := 15 * time.Second
				tlsRetry := 2 * time.Second
				Eventually(func() string {
					return aggregateEventMsgs(rabbitmqCluster, "TLSError")
				}, tlsEventTimeout, tlsRetry).Should(ContainSubstring("Failed to get TLS secret"))
				_, err := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(rabbitmqCluster.ChildResourceName("server"), metav1.GetOptions{})
				Expect(err).To(HaveOccurred())

				// create missing secret
				tlsSecret := corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "tls-secret-does-not-exist",
						Namespace: "rabbitmq-namespace",
					},
					StringData: map[string]string{
						"tls.crt": "this is a tls cert",
						"tls.key": "this is a tls key",
					},
				}
				_, err = clientSet.CoreV1().Secrets("rabbitmq-namespace").Create(&tlsSecret)
				Expect(err).NotTo(HaveOccurred())

				waitForClusterCreation(rabbitmqCluster, client)
				statefulSet(rabbitmqCluster)
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
					Replicas:        &one,
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
					Replicas:        &one,
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
					Replicas:        &one,
					Affinity:        affinity,
					ImagePullSecret: "rabbit-two-secret",
				},
			}
			Expect(client.Create(context.TODO(), rabbitmqCluster)).To(Succeed())
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

	Context("Client service configurations", func() {
		AfterEach(func() {
			Expect(client.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
			Expect(clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Delete(rabbitmqCluster.ChildResourceName("client"), &metav1.DeleteOptions{}))
		})

		It("creates the service type and annotations as configured in instance spec", func() {
			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbit-service-2",
					Namespace: "rabbit-service-2",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas:        &one,
					ImagePullSecret: "rabbit-service-secret",
				},
			}
			rabbitmqCluster.Spec.Service.Type = "LoadBalancer"
			rabbitmqCluster.Spec.Service.Annotations = map[string]string{"annotations": "cr-annotation"}

			Expect(client.Create(context.TODO(), rabbitmqCluster)).To(Succeed())

			clientSvc := service(rabbitmqCluster, "client")
			Expect(clientSvc.Spec.Type).Should(Equal(corev1.ServiceTypeLoadBalancer))
			Expect(clientSvc.Annotations).Should(HaveKeyWithValue("annotations", "cr-annotation"))
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
					Replicas:        &one,
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

			Expect(client.Create(context.TODO(), rabbitmqCluster)).To(Succeed())

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
					Replicas:        &one,
					ImagePullSecret: "rabbit-resource-secret",
				},
			}
			storageClassName := "my-storage-class"
			rabbitmqCluster.Spec.Persistence.StorageClassName = &storageClassName
			storage := k8sresource.MustParse("100Gi")
			rabbitmqCluster.Spec.Persistence.Storage = &storage
			Expect(client.Create(context.TODO(), rabbitmqCluster)).To(Succeed())

			sts := statefulSet(rabbitmqCluster)

			Expect(len(sts.Spec.VolumeClaimTemplates)).To(Equal(1))
			Expect(*sts.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(Equal("my-storage-class"))
			actualStorageCapacity := sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]
			Expect(actualStorageCapacity).To(Equal(k8sresource.MustParse("100Gi")))
		})
	})

	Context("Custom Resource updates", func() {
		var (
			rabbitmqCluster   *rabbitmqv1beta1.RabbitmqCluster
			clientServiceName string
			statefulSetName   string
		)
		BeforeEach(func() {
			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-cr-update",
					Namespace: "rabbitmq-cr-update",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas:        &one,
					ImagePullSecret: "rabbit-two-secret",
				},
			}
			clientServiceName = rabbitmqCluster.ChildResourceName("client")
			statefulSetName = rabbitmqCluster.ChildResourceName("server")

			Expect(client.Create(context.TODO(), rabbitmqCluster)).To(Succeed())
			waitForClusterCreation(rabbitmqCluster, client)
		})

		AfterEach(func() {
			Expect(client.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
			waitForClusterDeletion(rabbitmqCluster, client)
		})

		It("the service annotations are updated", func() {
			Expect(updateWithRetry(rabbitmqCluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.Service.Annotations = map[string]string{"test-key": "test-value"}
			})).To(Succeed())

			Eventually(func() map[string]string {
				clientServiceName := rabbitmqCluster.ChildResourceName("client")
				service, _ := clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(clientServiceName, metav1.GetOptions{})
				return service.Annotations
			}, 3).Should(HaveKeyWithValue("test-key", "test-value"))

			// verify that SuccessfulUpdate event is recorded for the client service
			Expect(aggregateEventMsgs(rabbitmqCluster, "SuccessfulUpdate")).To(
				ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.Service", rabbitmqCluster.ChildResourceName("client"))))
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

			Expect(updateWithRetry(rabbitmqCluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.Resources = expectedRequirements
			})).To(Succeed())

			Eventually(func() corev1.ResourceList {
				stsName := rabbitmqCluster.ChildResourceName("server")
				sts, err := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(stsName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				resourceRequirements = sts.Spec.Template.Spec.Containers[0].Resources
				return resourceRequirements.Requests
			}, 3).Should(HaveKeyWithValue(corev1.ResourceCPU, expectedRequirements.Requests[corev1.ResourceCPU]))
			Expect(resourceRequirements.Limits).To(HaveKeyWithValue(corev1.ResourceCPU, expectedRequirements.Limits[corev1.ResourceCPU]))

			Expect(resourceRequirements.Requests).To(HaveKeyWithValue(corev1.ResourceMemory, expectedRequirements.Requests[corev1.ResourceMemory]))
			Expect(resourceRequirements.Limits).To(HaveKeyWithValue(corev1.ResourceMemory, expectedRequirements.Limits[corev1.ResourceMemory]))

			// verify that SuccessfulUpdate event is recorded for the StatefulSet
			Expect(aggregateEventMsgs(rabbitmqCluster, "SuccessfulUpdate")).To(
				ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.StatefulSet", rabbitmqCluster.ChildResourceName("server"))))
		})

		It("the rabbitmq image is updated", func() {
			Expect(updateWithRetry(rabbitmqCluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.Image = "rabbitmq:3.8.0"
			})).To(Succeed())

			Eventually(func() string {
				stsName := rabbitmqCluster.ChildResourceName("server")
				sts, _ := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(stsName, metav1.GetOptions{})
				return sts.Spec.Template.Spec.Containers[0].Image
			}, 3).Should(Equal("rabbitmq:3.8.0"))
		})

		It("the rabbitmq ImagePullSecret is updated", func() {
			Expect(updateWithRetry(rabbitmqCluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.ImagePullSecret = "my-new-secret"
			})).To(Succeed())

			Eventually(func() []corev1.LocalObjectReference {
				stsName := rabbitmqCluster.ChildResourceName("server")
				sts, _ := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(stsName, metav1.GetOptions{})
				Expect(len(sts.Spec.Template.Spec.ImagePullSecrets)).To(Equal(1))
				return sts.Spec.Template.Spec.ImagePullSecrets
			}, 3).Should(ConsistOf(corev1.LocalObjectReference{Name: "my-new-secret"}))
		})

		It("labels are updated", func() {
			Expect(updateWithRetry(rabbitmqCluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Labels = make(map[string]string)
				r.Labels["foo"] = "bar"
			})).To(Succeed())

			Eventually(func() map[string]string {
				service, err := clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(clientServiceName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				return service.Labels
			}, 3).Should(HaveKeyWithValue("foo", "bar"))
			var sts *appsv1.StatefulSet
			Eventually(func() map[string]string {
				sts, _ = clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(statefulSetName, metav1.GetOptions{})
				return sts.Labels
			}, 3).Should(HaveKeyWithValue("foo", "bar"))
		})

		It("instance annotations are updated", func() {
			Expect(updateWithRetry(rabbitmqCluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Annotations = make(map[string]string)
				r.Annotations["anno-key"] = "anno-value"
			})).To(Succeed())

			Eventually(func() map[string]string {
				service, err := clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(rabbitmqCluster.ChildResourceName("headless"), metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				return service.Annotations
			}, 3).Should(HaveKeyWithValue("anno-key", "anno-value"))
			var sts *appsv1.StatefulSet
			Eventually(func() map[string]string {
				sts, _ = clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(statefulSetName, metav1.GetOptions{})
				return sts.Annotations
			}, 3).Should(HaveKeyWithValue("anno-key", "anno-value"))

			// verify that SuccessfulUpdate events are recorded for all child resources
			allEventMsgs := aggregateEventMsgs(rabbitmqCluster, "SuccessfulUpdate")
			Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.StatefulSet", rabbitmqCluster.ChildResourceName("server"))))
			Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.Service", rabbitmqCluster.ChildResourceName("client"))))
			Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.Service", rabbitmqCluster.ChildResourceName("headless"))))
			Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.ConfigMap", rabbitmqCluster.ChildResourceName("server-conf"))))
			Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.Secret", rabbitmqCluster.ChildResourceName("erlang-cookie"))))
			Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.Secret", rabbitmqCluster.ChildResourceName("admin"))))
			Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.ServiceAccount", rabbitmqCluster.ChildResourceName("server"))))
			Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.Role", rabbitmqCluster.ChildResourceName("endpoint-discovery"))))
			Expect(allEventMsgs).To(ContainSubstring(fmt.Sprintf("updated resource %s of Type *v1.RoleBinding", rabbitmqCluster.ChildResourceName("server"))))
		})

		It("service type is updated", func() {
			Expect(updateWithRetry(rabbitmqCluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.Service.Type = "NodePort"
			})).To(Succeed())

			Eventually(func() string {
				service, err := clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(rabbitmqCluster.ChildResourceName("client"), metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				return string(service.Spec.Type)
			}, 3).Should(Equal("NodePort"))
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

			Expect(updateWithRetry(rabbitmqCluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.Affinity = affinity
			})).To(Succeed())

			Eventually(func() *corev1.Affinity {
				sts, _ := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(statefulSetName, metav1.GetOptions{})
				return sts.Spec.Template.Spec.Affinity
			}, 3).Should(Equal(affinity))

			Expect(client.Get(
				context.TODO(),
				types.NamespacedName{Name: rabbitmqCluster.Name, Namespace: rabbitmqCluster.Namespace},
				rabbitmqCluster,
			)).To(Succeed())

			affinity = nil
			Expect(updateWithRetry(rabbitmqCluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.Affinity = affinity
			})).To(Succeed())
			Eventually(func() *corev1.Affinity {
				sts, _ := clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(statefulSetName, metav1.GetOptions{})
				return sts.Spec.Template.Spec.Affinity
			}, 3).Should(BeNil())
		})
	})

	Context("Recreate child resources after deletion", func() {
		var (
			clientServiceName   string
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
					Replicas:        &one,
					ImagePullSecret: "rabbit-two-secret",
				},
			}
			clientServiceName = rabbitmqCluster.ChildResourceName("client")
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

			oldClientSvc := service(rabbitmqCluster, "client")

			oldHeadlessSvc := service(rabbitmqCluster, "headless")

			oldSts := statefulSet(rabbitmqCluster)

			Expect(clientSet.AppsV1().StatefulSets(namespace).Delete(stsName, &metav1.DeleteOptions{})).NotTo(HaveOccurred())
			Expect(clientSet.CoreV1().ConfigMaps(namespace).Delete(configMapName, &metav1.DeleteOptions{})).NotTo(HaveOccurred())
			Expect(clientSet.CoreV1().Services(namespace).Delete(clientServiceName, &metav1.DeleteOptions{})).NotTo(HaveOccurred())
			Expect(clientSet.CoreV1().Services(namespace).Delete(headlessServiceName, &metav1.DeleteOptions{})).NotTo(HaveOccurred())

			Eventually(func() bool {
				sts, err := clientSet.AppsV1().StatefulSets(namespace).Get(stsName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return string(sts.UID) != string(oldSts.UID)
			}, 5).Should(BeTrue())

			Eventually(func() bool {
				clientSvc, err := clientSet.CoreV1().Services(namespace).Get(clientServiceName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return string(clientSvc.UID) != string(oldClientSvc.UID)
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

	Context("RabbitMQ CR ReconcileSuccess condition", func() {
		var (
			rabbitmqResource *rabbitmqv1beta1.RabbitmqCluster
			crName           string
		)

		BeforeEach(func() {
			crName = "irreconcilable"
			rabbitmqResource = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      crName,
					Namespace: "default",
				},
			}
			rabbitmqResource.Spec.Replicas = &one
		})

		It("exposes ReconcileSuccess condition", func() {
			By("setting to False when spec is not valid", func() {
				// Annotations must end in alphanumeric character. However KubeAPI will accept this manifest
				rabbitmqResource.Spec.Service.Annotations = map[string]string{"thisIs-": "notValidForK8s"}
				Expect(client.Create(context.Background(), rabbitmqResource)).To(Succeed())
				waitForClusterCreation(rabbitmqResource, client)

				Eventually(func() string {
					someRabbit := &rabbitmqv1beta1.RabbitmqCluster{}
					Expect(client.Get(context.Background(), runtimeClient.ObjectKey{
						Name:      crName,
						Namespace: "default",
					}, someRabbit)).To(Succeed())

					for i := range someRabbit.Status.Conditions {
						if someRabbit.Status.Conditions[i].Type == status.ReconcileSuccess {
							return fmt.Sprintf("ReconcileSuccess status: %s", someRabbit.Status.Conditions[i].Status)
						}
					}
					return "ReconcileSuccess status: condition not present"
				}, 5).Should(Equal("ReconcileSuccess status: False"))
			})

			By("transitioning to True when a valid spec in updated", func() {
				// We have to Get() the CR again because Reconcile() changes the object
				// If we try to Update() without getting the latest version of the CR
				// We are very likely to hit a Conflict error
				Expect(client.Get(context.Background(), runtimeClient.ObjectKey{
					Name:      crName,
					Namespace: "default",
				}, rabbitmqResource)).To(Succeed())
				rabbitmqResource.Spec.Service.Annotations = map[string]string{"thisIs": "valid"}
				Expect(client.Update(context.Background(), rabbitmqResource)).To(Succeed())

				Eventually(func() string {
					someRabbit := &rabbitmqv1beta1.RabbitmqCluster{}
					Expect(client.Get(context.Background(), runtimeClient.ObjectKey{
						Name:      crName,
						Namespace: "default",
					}, someRabbit)).To(Succeed())

					for i := range someRabbit.Status.Conditions {
						if someRabbit.Status.Conditions[i].Type == status.ReconcileSuccess {
							return fmt.Sprintf("ReconcileSuccess status: %s", someRabbit.Status.Conditions[i].Status)
						}
					}
					return "ReconcileSuccess status: condition not present"
				}, 5).Should(Equal("ReconcileSuccess status: True"))
			})
		})
	})

	Context("Stateful Set Override", func() {
		var (
			stsOverrideCluster *rabbitmqv1beta1.RabbitmqCluster
			q, myStorage       k8sresource.Quantity
			storageClassName   string
		)

		BeforeEach(func() {
			storageClassName = "my-storage-class"
			myStorage = k8sresource.MustParse("100Gi")
			q, _ = k8sresource.ParseQuantity("10Gi")
			ten := int32(10)
			stsOverrideCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-sts-override",
					Namespace: "rabbitmq-sts-override",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: &ten,
					Override: rabbitmqv1beta1.RabbitmqClusterOverrideSpec{
						StatefulSet: &rabbitmqv1beta1.StatefulSet{
							Spec: &rabbitmqv1beta1.StatefulSetSpec{
								VolumeClaimTemplates: []rabbitmqv1beta1.PersistentVolumeClaim{
									{
										EmbeddedObjectMeta: rabbitmqv1beta1.EmbeddedObjectMeta{
											Name:      "persistence",
											Namespace: "rabbitmq-sts-override",
											Labels: map[string]string{
												"app.kubernetes.io/name": "rabbitmq-sts-override",
											},
											Annotations: map[string]string{},
										},
										Spec: corev1.PersistentVolumeClaimSpec{
											AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
											Resources: corev1.ResourceRequirements{
												Requests: map[corev1.ResourceName]k8sresource.Quantity{
													corev1.ResourceStorage: q,
												},
											},
										},
									},
									{
										EmbeddedObjectMeta: rabbitmqv1beta1.EmbeddedObjectMeta{
											Name:      "disk-2",
											Namespace: "rabbitmq-sts-override",
											Labels: map[string]string{
												"app.kubernetes.io/name": "rabbitmq-sts-override",
											},
										},
										Spec: corev1.PersistentVolumeClaimSpec{
											Resources: corev1.ResourceRequirements{
												Requests: corev1.ResourceList{
													corev1.ResourceStorage: myStorage,
												},
											},
											StorageClassName: &storageClassName,
										},
									},
								},
								Template: &rabbitmqv1beta1.PodTemplateSpec{
									Spec: &corev1.PodSpec{
										HostNetwork: false,
										Volumes: []corev1.Volume{
											{
												Name: "additional-config",
												VolumeSource: corev1.VolumeSource{
													ConfigMap: &corev1.ConfigMapVolumeSource{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: "additional-config-confmap",
														},
													},
												},
											},
										},
										Containers: []corev1.Container{
											{
												Name:  "additional-container",
												Image: "my-great-image",
											},
										},
									},
								},
							},
						},
					},
				},
			}
			Expect(client.Create(context.Background(), stsOverrideCluster)).To(Succeed())
			waitForClusterCreation(stsOverrideCluster, client)
		})

		AfterEach(func() {
			Expect(client.Delete(context.Background(), stsOverrideCluster)).To(Succeed())
			waitForClusterDeletion(stsOverrideCluster, client)
		})

		It("creates a StatefulSet with the override applied", func() {
			sts := statefulSet(stsOverrideCluster)
			myStorage := k8sresource.MustParse("100Gi")
			volumeMode := corev1.PersistentVolumeMode("Filesystem")
			defaultMode := int32(420)

			Expect(sts.ObjectMeta.Labels).To(Equal(map[string]string{
				"app.kubernetes.io/name":      "rabbitmq-sts-override",
				"app.kubernetes.io/component": "rabbitmq",
				"app.kubernetes.io/part-of":   "rabbitmq",
			}))

			Expect(sts.Spec.ServiceName).To(Equal("rabbitmq-sts-override-rabbitmq-headless"))
			Expect(sts.Spec.Selector.MatchLabels).To(Equal(map[string]string{
				"app.kubernetes.io/name": "rabbitmq-sts-override",
			}))

			Expect(len(sts.Spec.VolumeClaimTemplates)).To(Equal(2))

			Expect(sts.Spec.VolumeClaimTemplates[0].ObjectMeta.Name).To(Equal("persistence"))
			Expect(sts.Spec.VolumeClaimTemplates[0].ObjectMeta.Namespace).To(Equal("rabbitmq-sts-override"))
			Expect(sts.Spec.VolumeClaimTemplates[0].ObjectMeta.Labels).To(Equal(
				map[string]string{
					"app.kubernetes.io/name": "rabbitmq-sts-override",
				}))
			Expect(sts.Spec.VolumeClaimTemplates[0].OwnerReferences[0].Name).To(Equal("rabbitmq-sts-override"))
			Expect(sts.Spec.VolumeClaimTemplates[0].Spec).To(Equal(
				corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					VolumeMode:  &volumeMode,
					Resources: corev1.ResourceRequirements{
						Requests: map[corev1.ResourceName]k8sresource.Quantity{
							corev1.ResourceStorage: q,
						},
					},
				}))

			Expect(sts.Spec.VolumeClaimTemplates[1].ObjectMeta.Name).To(Equal("disk-2"))
			Expect(sts.Spec.VolumeClaimTemplates[1].ObjectMeta.Namespace).To(Equal("rabbitmq-sts-override"))
			Expect(sts.Spec.VolumeClaimTemplates[1].ObjectMeta.Labels).To(Equal(
				map[string]string{
					"app.kubernetes.io/name": "rabbitmq-sts-override",
				}))
			Expect(sts.Spec.VolumeClaimTemplates[1].OwnerReferences[0].Name).To(Equal("rabbitmq-sts-override"))
			Expect(sts.Spec.VolumeClaimTemplates[1].Spec).To(Equal(
				corev1.PersistentVolumeClaimSpec{
					VolumeMode:       &volumeMode,
					StorageClassName: &storageClassName,
					Resources: corev1.ResourceRequirements{
						Requests: map[corev1.ResourceName]k8sresource.Quantity{
							corev1.ResourceStorage: myStorage,
						},
					},
				}))

			Expect(sts.Spec.Template.Spec.HostNetwork).To(BeFalse())
			Expect(sts.Spec.Template.Spec.Volumes).To(ConsistOf(
				corev1.Volume{
					Name: "additional-config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "additional-config-confmap",
							},
							DefaultMode: &defaultMode,
						},
					},
				},
				corev1.Volume{
					Name: "rabbitmq-admin",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							DefaultMode: &defaultMode,
							SecretName:  "rabbitmq-sts-override-rabbitmq-admin",
							Items: []corev1.KeyToPath{
								{
									Key:  "username",
									Path: "username",
								},
								{
									Key:  "password",
									Path: "password",
								},
							},
						},
					},
				},
				corev1.Volume{
					Name: "server-conf",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							DefaultMode: &defaultMode,
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "rabbitmq-sts-override-rabbitmq-server-conf",
							},
						},
					},
				},
				corev1.Volume{
					Name: "rabbitmq-etc",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				corev1.Volume{
					Name: "rabbitmq-erlang-cookie",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				corev1.Volume{
					Name: "erlang-cookie-secret",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							DefaultMode: &defaultMode,
							SecretName:  "rabbitmq-sts-override-rabbitmq-erlang-cookie",
						},
					},
				},
				corev1.Volume{
					Name: "pod-info",
					VolumeSource: corev1.VolumeSource{
						DownwardAPI: &corev1.DownwardAPIVolumeSource{
							DefaultMode: &defaultMode,
							Items: []corev1.DownwardAPIVolumeFile{
								{
									Path: "skipPreStopChecks",
									FieldRef: &corev1.ObjectFieldSelector{
										APIVersion: "v1",
										FieldPath:  fmt.Sprintf("metadata.labels['%s']", "skipPreStopChecks"),
									},
								},
							},
						},
					},
				}))

			Expect(extractContainer(sts.Spec.Template.Spec.Containers, "additional-container").Image).To(Equal("my-great-image"))
		})

		It("updates", func() {
			five := int32(5)

			Expect(updateWithRetry(stsOverrideCluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				stsOverrideCluster.Spec.Override.StatefulSet.Spec.Replicas = &five
				stsOverrideCluster.Spec.Override.StatefulSet.Spec.Template.Spec.Containers = []corev1.Container{
					{
						Name:  "additional-container-2",
						Image: "my-great-image-2",
					},
				}
			})).To(Succeed())

			Eventually(func() int32 {
				sts := statefulSet(stsOverrideCluster)
				return *sts.Spec.Replicas
			}, 3).Should(Equal(int32(5)))

			Eventually(func() string {
				sts := statefulSet(stsOverrideCluster)
				c := extractContainer(sts.Spec.Template.Spec.Containers, "additional-container-2")
				return c.Image
			}, 3).Should(Equal("my-great-image-2"))
		})
	})

	Context("Client Service Override", func() {
		var (
			svcOverrideCluster *rabbitmqv1beta1.RabbitmqCluster
		)

		BeforeEach(func() {
			svcOverrideCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc-override",
					Namespace: "svc-override",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: &one,
					Service: rabbitmqv1beta1.RabbitmqClusterServiceSpec{
						Type: "LoadBalancer",
					},
					Override: rabbitmqv1beta1.RabbitmqClusterOverrideSpec{
						ClientService: &rabbitmqv1beta1.ClientService{
							Spec: &corev1.ServiceSpec{
								Ports: []corev1.ServicePort{
									{
										Protocol: corev1.ProtocolTCP,
										Port:     15535,
										Name:     "additional-port",
									},
								},
								Selector: map[string]string{
									"a-selector": "a-label",
								},
								Type:                     "ClusterIP",
								SessionAffinity:          "ClientIP",
								PublishNotReadyAddresses: false,
							},
						},
					},
				},
			}

			Expect(client.Create(context.Background(), svcOverrideCluster)).To(Succeed())
			waitForClusterCreation(svcOverrideCluster, client)
		})

		AfterEach(func() {
			Expect(client.Delete(context.Background(), svcOverrideCluster)).To(Succeed())
			waitForClusterDeletion(svcOverrideCluster, client)
		})

		It("creates a Client Service with the override applied", func() {
			amqpTargetPort := intstr.IntOrString{IntVal: int32(5672)}
			managementTargetPort := intstr.IntOrString{IntVal: int32(15672)}
			additionalTargetPort := intstr.IntOrString{IntVal: int32(15535)}
			svc := service(svcOverrideCluster, "client")
			Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
			Expect(svc.Spec.Ports).To(ConsistOf(
				corev1.ServicePort{
					Name:       "amqp",
					Port:       5672,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: amqpTargetPort,
				},
				corev1.ServicePort{
					Name:       "management",
					Port:       15672,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: managementTargetPort,
				},
				corev1.ServicePort{
					Protocol:   corev1.ProtocolTCP,
					Port:       15535,
					Name:       "additional-port",
					TargetPort: additionalTargetPort,
				},
			))
			Expect(svc.Spec.Selector).To(Equal(map[string]string{"a-selector": "a-label", "app.kubernetes.io/name": "svc-override"}))
			Expect(svc.Spec.SessionAffinity).To(Equal(corev1.ServiceAffinityClientIP))
			Expect(svc.Spec.PublishNotReadyAddresses).To(BeFalse())
		})

		It("updates", func() {
			Expect(updateWithRetry(svcOverrideCluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				svcOverrideCluster.Spec.Override.ClientService.Spec.Type = "LoadBalancer"
			})).To(Succeed())

			Eventually(func() corev1.ServiceType {
				svc := service(svcOverrideCluster, "client")
				return svc.Spec.Type
			}, 5).Should(Equal(corev1.ServiceTypeLoadBalancer))
		})
	})
})

func extractContainer(containers []corev1.Container, containerName string) corev1.Container {
	for _, container := range containers {
		if container.Name == containerName {
			return container
		}
	}

	return corev1.Container{}
}

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
	}, 10).Should(Succeed())
	return sts
}

func service(rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster, svcName string) *corev1.Service {
	serviceName := rabbitmqCluster.ChildResourceName(svcName)
	var svc *corev1.Service
	EventuallyWithOffset(1, func() error {
		var err error
		svc, err = clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(serviceName, metav1.GetOptions{})
		return err
	}, 10).Should(Succeed())
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
			return "not ready"
		}

		return "ready"

	}, ClusterCreationTimeout, 1*time.Second).Should(Equal("ready"))

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
	}, ClusterDeletionTimeout, 1*time.Second).Should(BeTrue())

}
