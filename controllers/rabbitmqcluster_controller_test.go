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
	"time"

	"k8s.io/apimachinery/pkg/util/intstr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/resource"
	"github.com/rabbitmq/cluster-operator/internal/status"
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

var _ = Describe("RabbitmqClusterController", func() {

	var (
		cluster          *rabbitmqv1beta1.RabbitmqCluster
		one              int32 = 1
		defaultNamespace       = "default"
		ctx                    = context.Background()
		updateWithRetry        = func(cr *rabbitmqv1beta1.RabbitmqCluster, mutateFn func(r *rabbitmqv1beta1.RabbitmqCluster)) error {
			return retry.RetryOnConflict(retry.DefaultRetry, func() error {
				objKey, err := runtimeClient.ObjectKeyFromObject(cr)
				if err != nil {
					return err
				}

				if err := client.Get(ctx, objKey, cr); err != nil {
					return err
				}

				mutateFn(cr)
				return client.Update(ctx, cr)
			})
		}
	)

	Context("default settings", func() {
		BeforeEach(func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-one",
					Namespace: defaultNamespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: &one,
				},
			}

			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)
		})

		AfterEach(func() {
			Expect(client.Delete(ctx, cluster)).To(Succeed())
			Eventually(func() bool {
				rmq := &rabbitmqv1beta1.RabbitmqCluster{}
				err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
				return apierrors.IsNotFound(err)
			}, 5).Should(BeTrue())
		})

		It("works", func() {
			By("creating a statefulset with default configurations", func() {
				sts := statefulSet(ctx, cluster)

				Expect(sts.Name).To(Equal(cluster.ChildResourceName("server")))
				Expect(sts.Spec.Template.Spec.ImagePullSecrets).To(BeEmpty())

				Expect(len(sts.Spec.VolumeClaimTemplates)).To(Equal(1))
				Expect(sts.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(BeNil())
			})

			By("creating the server conf configmap", func() {
				configMapName := cluster.ChildResourceName("server-conf")
				configMap, err := clientSet.CoreV1().ConfigMaps(cluster.Namespace).Get(ctx, configMapName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(configMap.Name).To(Equal(configMapName))
				Expect(configMap.OwnerReferences[0].Name).To(Equal(cluster.Name))
			})

			By("creating the plugins conf configmap", func() {
				configMapName := cluster.ChildResourceName("plugins-conf")
				configMap, err := clientSet.CoreV1().ConfigMaps(cluster.Namespace).Get(ctx, configMapName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(configMap.Name).To(Equal(configMapName))
				Expect(configMap.OwnerReferences[0].Name).To(Equal(cluster.Name))
			})

			By("creating a rabbitmq admin secret", func() {
				secretName := cluster.ChildResourceName("admin")
				secret, err := clientSet.CoreV1().Secrets(cluster.Namespace).Get(ctx, secretName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Name).To(Equal(secretName))
				Expect(secret.OwnerReferences[0].Name).To(Equal(cluster.Name))
			})

			By("creating an erlang cookie secret", func() {
				secretName := cluster.ChildResourceName("erlang-cookie")
				secret, err := clientSet.CoreV1().Secrets(cluster.Namespace).Get(ctx, secretName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Name).To(Equal(secretName))
				Expect(secret.OwnerReferences[0].Name).To(Equal(cluster.Name))
			})

			By("creating a rabbitmq client service", func() {
				svc := service(ctx, cluster, "client")
				Expect(svc.Name).To(Equal(cluster.ChildResourceName("client")))
				Expect(svc.OwnerReferences[0].Name).To(Equal(cluster.Name))
				Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
			})

			By("creating a rabbitmq headless service", func() {
				svc := service(ctx, cluster, "headless")
				Expect(svc.Name).To(Equal(cluster.ChildResourceName("headless")))
				Expect(svc.OwnerReferences[0].Name).To(Equal(cluster.Name))
			})

			By("creating a service account", func() {
				serviceAccountName := cluster.ChildResourceName("server")
				serviceAccount, err := clientSet.CoreV1().ServiceAccounts(cluster.Namespace).Get(ctx, serviceAccountName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(serviceAccount.Name).To(Equal(serviceAccountName))
				Expect(serviceAccount.OwnerReferences[0].Name).To(Equal(cluster.Name))
			})

			By("creating a role", func() {
				roleName := cluster.ChildResourceName("peer-discovery")
				role, err := clientSet.RbacV1().Roles(cluster.Namespace).Get(ctx, roleName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(role.Name).To(Equal(roleName))
				Expect(role.OwnerReferences[0].Name).To(Equal(cluster.Name))
			})

			By("creating a role binding", func() {
				roleBindingName := cluster.ChildResourceName("server")
				roleBinding, err := clientSet.RbacV1().RoleBindings(cluster.Namespace).Get(ctx, roleBindingName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(roleBinding.Name).To(Equal(roleBindingName))
				Expect(roleBinding.OwnerReferences[0].Name).To(Equal(cluster.Name))
			})
			By("recording SuccessfulCreate events for all child resources", func() {
				allEventMsgs := aggregateEventMsgs(ctx, cluster, "SuccessfulCreate")
				Expect(allEventMsgs).To(ContainSubstring("created resource %s of Type *v1.StatefulSet", cluster.ChildResourceName("server")))
				Expect(allEventMsgs).To(ContainSubstring("created resource %s of Type *v1.Service", cluster.ChildResourceName("client")))
				Expect(allEventMsgs).To(ContainSubstring("created resource %s of Type *v1.Service", cluster.ChildResourceName("headless")))
				Expect(allEventMsgs).To(ContainSubstring("created resource %s of Type *v1.ConfigMap", cluster.ChildResourceName("plugins-conf")))
				Expect(allEventMsgs).To(ContainSubstring("created resource %s of Type *v1.ConfigMap", cluster.ChildResourceName("server-conf")))
				Expect(allEventMsgs).To(ContainSubstring("created resource %s of Type *v1.Secret", cluster.ChildResourceName("erlang-cookie")))
				Expect(allEventMsgs).To(ContainSubstring("created resource %s of Type *v1.Secret", cluster.ChildResourceName("admin")))
				Expect(allEventMsgs).To(ContainSubstring("created resource %s of Type *v1.ServiceAccount", cluster.ChildResourceName("server")))
				Expect(allEventMsgs).To(ContainSubstring("created resource %s of Type *v1.Role", cluster.ChildResourceName("peer-discovery")))
				Expect(allEventMsgs).To(ContainSubstring("created resource %s of Type *v1.RoleBinding", cluster.ChildResourceName("server")))
			})

			By("adding the deletion finalizer", func() {
				rmq := &rabbitmqv1beta1.RabbitmqCluster{}
				Eventually(func() string {
					err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
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
					err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
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
				Expect(secretRef.Keys).To(HaveKeyWithValue("username", "username"))
				Expect(secretRef.Keys).To(HaveKeyWithValue("password", "password"))
			})

			By("setting the client service details in the custom resource status", func() {
				rmq := &rabbitmqv1beta1.RabbitmqCluster{}
				serviceRef := &rabbitmqv1beta1.RabbitmqClusterServiceReference{}
				Eventually(func() *rabbitmqv1beta1.RabbitmqClusterServiceReference {
					err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
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

	Context("Mutual TLS", func() {
		AfterEach(func() {
			Expect(client.Delete(ctx, cluster)).To(Succeed())
			Eventually(func() bool {
				rmq := &rabbitmqv1beta1.RabbitmqCluster{}
				err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
				return apierrors.IsNotFound(err)
			}, 5).Should(BeTrue())
		})

		Context("Mutual TLS with single secret", func() {
			It("Deploys successfully", func() {
				tlsSecretWithCACert(ctx, "tls-secret", defaultNamespace, "caCERT")
				tlsSpec := rabbitmqv1beta1.TLSSpec{
					SecretName:   "tls-secret",
					CaSecretName: "tls-secret",
					CaCertName:   "caCERT",
				}
				cluster = rabbitmqClusterWithTLS(ctx, "mutual-tls-success", defaultNamespace, tlsSpec)
				waitForClusterCreation(ctx, cluster, client)

				sts, err := clientSet.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, cluster.ChildResourceName("server"), metav1.GetOptions{})
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
				tlsSecretWithCACert(ctx, "tls-secret-missing", defaultNamespace, "ca.c")
				tlsSpec := rabbitmqv1beta1.TLSSpec{
					SecretName:   "tls-secret-missing",
					CaSecretName: "tls-secret-missing",
					CaCertName:   "ca.crt",
				}
				cluster = rabbitmqClusterWithTLS(ctx, "tls-secret-missing", defaultNamespace, tlsSpec)

				verifyError(ctx, cluster, fmt.Sprintf("The TLS secret tls-secret-missing in namespace %s must have the field ca.crt", defaultNamespace))
			})
		})

		Context("Mutual TLS with a seperate CA certificate secret", func() {
			It("Does not deploy the RabbitmqCluster, and retries every 10 seconds", func() {
				tlsSecretWithoutCACert(ctx, "rabbitmq-tls-secret-does-not-exist", defaultNamespace)

				tlsSpec := rabbitmqv1beta1.TLSSpec{
					SecretName:   "rabbitmq-tls-secret-does-not-exist",
					CaSecretName: "ca-cert-secret",
					CaCertName:   "ca.crt",
				}
				cluster = rabbitmqClusterWithTLS(ctx, "rabbitmq-tls-secret-does-not-exist", defaultNamespace, tlsSpec)
				verifyError(ctx, cluster, "Failed to get CA certificate secret")

				_, err := clientSet.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, cluster.ChildResourceName("server"), metav1.GetOptions{})
				Expect(err).To(HaveOccurred())

				// create missing secret
				caData := map[string]string{
					"ca.crt": "this is a ca cert",
				}
				_, err = createSecret(ctx, "ca-cert-secret", defaultNamespace, caData)
				Expect(err).NotTo(HaveOccurred())

				waitForClusterCreation(ctx, cluster, client)
				statefulSet(ctx, cluster)
			})

			It("Does not deploy if the cert name does not match the contents of the secret", func() {
				tlsSecretWithoutCACert(ctx, "tls-secret", defaultNamespace)
				caData := map[string]string{
					"cacrt": "this is a ca cert",
				}

				_, err := createSecret(ctx, "ca-cert-secret-invalid", defaultNamespace, caData)
				if !apierrors.IsAlreadyExists(err) {
					Expect(err).NotTo(HaveOccurred())
				}

				tlsSpec := rabbitmqv1beta1.TLSSpec{
					SecretName:   "tls-secret",
					CaSecretName: "ca-cert-secret-invalid",
					CaCertName:   "ca.crt",
				}
				cluster = rabbitmqClusterWithTLS(ctx, "rabbitmq-mutual-tls-missing", defaultNamespace, tlsSpec)
				verifyError(ctx, cluster, fmt.Sprintf("The TLS secret ca-cert-secret-invalid in namespace %s must have the field ca.crt", defaultNamespace))
			})
		})
	})

	Context("TLS set on the instance", func() {
		AfterEach(func() {
			Expect(client.Delete(ctx, cluster)).To(Succeed())
			Eventually(func() bool {
				rmq := &rabbitmqv1beta1.RabbitmqCluster{}
				err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
				return apierrors.IsNotFound(err)
			}, 5).Should(BeTrue())
		})

		BeforeEach(func() {
			tlsSecretWithoutCACert(ctx, "tls-secret", defaultNamespace)
		})

		It("Deploys successfully", func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-tls",
					Namespace: defaultNamespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: &one,
					TLS: rabbitmqv1beta1.TLSSpec{
						SecretName: "tls-secret",
					},
				},
			}

			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)
		})

		When("the TLS secret does not have the expected keys - tls.crt, or tls.key", func() {
			BeforeEach(func() {
				secretData := map[string]string{
					"somekey": "someval",
					"tls.key": "this is a tls key",
				}
				_, err := createSecret(ctx, "rabbitmq-tls-malformed", defaultNamespace, secretData)

				if !apierrors.IsAlreadyExists(err) {
					Expect(err).NotTo(HaveOccurred())
				}

				tlsSpec := rabbitmqv1beta1.TLSSpec{
					SecretName: "rabbitmq-tls-malformed",
				}
				cluster = rabbitmqClusterWithTLS(ctx, "rabbitmq-tls-malformed", defaultNamespace, tlsSpec)
			})

			It("fails to deploy the RabbitmqCluster", func() {
				verifyError(ctx, cluster, fmt.Sprintf("The TLS secret rabbitmq-tls-malformed in namespace %s must have the fields tls.crt and tls.key", defaultNamespace))
			})
		})

		When("the TLS secret does not exist", func() {
			It("fails to deploy the RabbitmqCluster until the secret is detected", func() {

				tlsSpec := rabbitmqv1beta1.TLSSpec{
					SecretName: "tls-secret-does-not-exist",
				}
				cluster = rabbitmqClusterWithTLS(ctx, "rabbitmq-tls-secret-does-not-exist", defaultNamespace, tlsSpec)

				verifyError(ctx, cluster, "Failed to get TLS secret")

				_, err := clientSet.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, cluster.ChildResourceName("server"), metav1.GetOptions{})
				Expect(err).To(HaveOccurred())

				// create missing secret
				secretData := map[string]string{
					"tls.crt": "this is a tls cert",
					"tls.key": "this is a tls key",
				}
				_, err = createSecret(ctx, "tls-secret-does-not-exist", defaultNamespace, secretData)
				Expect(err).NotTo(HaveOccurred())

				waitForClusterCreation(ctx, cluster, client)
				statefulSet(ctx, cluster)
			})
		})

	})

	Context("Annotations set on the instance", func() {
		BeforeEach(func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-annotations",
					Namespace: defaultNamespace,
					Annotations: map[string]string{
						"my-annotation": "this-annotation",
					},
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas:        &one,
					ImagePullSecret: "rabbit-two-secret",
				},
			}

			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)
		})

		AfterEach(func() {
			Expect(client.Delete(ctx, cluster)).To(Succeed())
		})

		It("adds annotations to child resources", func() {
			headlessSvc := service(ctx, cluster, "headless")
			Expect(headlessSvc.Annotations).Should(HaveKeyWithValue("my-annotation", "this-annotation"))

			sts := statefulSet(ctx, cluster)
			Expect(sts.Annotations).Should(HaveKeyWithValue("my-annotation", "this-annotation"))
		})

	})

	Context("ImagePullSecret configure on the instance", func() {
		BeforeEach(func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-two",
					Namespace: defaultNamespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas:        &one,
					ImagePullSecret: "rabbit-two-secret",
				},
			}

			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)
		})

		AfterEach(func() {
			Expect(client.Delete(ctx, cluster)).To(Succeed())
		})

		It("configures the imagePullSecret on sts correctly", func() {
			By("using the instance spec secret", func() {
				sts := statefulSet(ctx, cluster)
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
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-affinity",
					Namespace: defaultNamespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas:        &one,
					Affinity:        affinity,
					ImagePullSecret: "rabbit-two-secret",
				},
			}
			Expect(client.Create(ctx, cluster)).To(Succeed())
		})

		AfterEach(func() {
			Expect(client.Delete(ctx, cluster)).To(Succeed())
		})

		It("adds the affinity rules to pod spec", func() {
			sts := statefulSet(ctx, cluster)
			podSpecAffinity := sts.Spec.Template.Spec.Affinity
			Expect(podSpecAffinity).To(Equal(affinity))
		})
	})

	Context("Client service configurations", func() {
		AfterEach(func() {
			Expect(client.Delete(ctx, cluster)).To(Succeed())
			Expect(clientSet.CoreV1().Services(cluster.Namespace).Delete(ctx, cluster.ChildResourceName("client"), metav1.DeleteOptions{}))
		})

		It("creates the service type and annotations as configured in instance spec", func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbit-service-2",
					Namespace: defaultNamespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas:        &one,
					ImagePullSecret: "rabbit-service-secret",
				},
			}
			cluster.Spec.Service.Type = "LoadBalancer"
			cluster.Spec.Service.Annotations = map[string]string{"annotations": "cr-annotation"}

			Expect(client.Create(ctx, cluster)).To(Succeed())

			clientSvc := service(ctx, cluster, "client")
			Expect(clientSvc.Spec.Type).Should(Equal(corev1.ServiceTypeLoadBalancer))
			Expect(clientSvc.Annotations).Should(HaveKeyWithValue("annotations", "cr-annotation"))
		})
	})

	Context("Resource requirements configurations", func() {
		AfterEach(func() {
			Expect(client.Delete(ctx, cluster)).To(Succeed())
		})

		It("uses resource requirements from instance spec when provided", func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbit-resource-2",
					Namespace: defaultNamespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas:        &one,
					ImagePullSecret: "rabbit-resource-secret",
				},
			}
			cluster.Spec.Resources = &corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]k8sresource.Quantity{
					corev1.ResourceMemory: k8sresource.MustParse("4Gi"),
				},
				Requests: map[corev1.ResourceName]k8sresource.Quantity{
					corev1.ResourceMemory: k8sresource.MustParse("4Gi"),
				},
			}

			Expect(client.Create(ctx, cluster)).To(Succeed())

			sts := statefulSet(ctx, cluster)

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
			Expect(client.Delete(ctx, cluster)).To(Succeed())
		})

		It("creates the RabbitmqCluster with the specified storage from instance spec", func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbit-persistence-1",
					Namespace: defaultNamespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas:        &one,
					ImagePullSecret: "rabbit-resource-secret",
				},
			}
			storageClassName := "my-storage-class"
			cluster.Spec.Persistence.StorageClassName = &storageClassName
			storage := k8sresource.MustParse("100Gi")
			cluster.Spec.Persistence.Storage = &storage
			Expect(client.Create(ctx, cluster)).To(Succeed())

			sts := statefulSet(ctx, cluster)

			Expect(len(sts.Spec.VolumeClaimTemplates)).To(Equal(1))
			Expect(*sts.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(Equal("my-storage-class"))
			actualStorageCapacity := sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]
			Expect(actualStorageCapacity).To(Equal(k8sresource.MustParse("100Gi")))
		})
	})

	Context("Custom Resource updates", func() {
		var (
			clientServiceName string
			statefulSetName   string
		)
		BeforeEach(func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-cr-update",
					Namespace: defaultNamespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas:        &one,
					ImagePullSecret: "rabbit-two-secret",
				},
			}
			clientServiceName = cluster.ChildResourceName("client")
			statefulSetName = cluster.ChildResourceName("server")

			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)
		})

		AfterEach(func() {
			Expect(client.Delete(ctx, cluster)).To(Succeed())
			waitForClusterDeletion(ctx, cluster, client)
		})

		It("the service annotations are updated", func() {
			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.Service.Annotations = map[string]string{"test-key": "test-value"}
			})).To(Succeed())

			Eventually(func() map[string]string {
				clientServiceName := cluster.ChildResourceName("client")
				service, _ := clientSet.CoreV1().Services(cluster.Namespace).Get(ctx, clientServiceName, metav1.GetOptions{})
				return service.Annotations
			}, 3).Should(HaveKeyWithValue("test-key", "test-value"))

			// verify that SuccessfulUpdate event is recorded for the client service
			Expect(aggregateEventMsgs(ctx, cluster, "SuccessfulUpdate")).To(
				ContainSubstring("updated resource %s of Type *v1.Service", cluster.ChildResourceName("client")))
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

			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.Resources = expectedRequirements
			})).To(Succeed())

			Eventually(func() corev1.ResourceList {
				stsName := cluster.ChildResourceName("server")
				sts, err := clientSet.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, stsName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				resourceRequirements = sts.Spec.Template.Spec.Containers[0].Resources
				return resourceRequirements.Requests
			}, 3).Should(HaveKeyWithValue(corev1.ResourceCPU, expectedRequirements.Requests[corev1.ResourceCPU]))
			Expect(resourceRequirements.Limits).To(HaveKeyWithValue(corev1.ResourceCPU, expectedRequirements.Limits[corev1.ResourceCPU]))

			Expect(resourceRequirements.Requests).To(HaveKeyWithValue(corev1.ResourceMemory, expectedRequirements.Requests[corev1.ResourceMemory]))
			Expect(resourceRequirements.Limits).To(HaveKeyWithValue(corev1.ResourceMemory, expectedRequirements.Limits[corev1.ResourceMemory]))

			// verify that SuccessfulUpdate event is recorded for the StatefulSet
			Expect(aggregateEventMsgs(ctx, cluster, "SuccessfulUpdate")).To(
				ContainSubstring("updated resource %s of Type *v1.StatefulSet", cluster.ChildResourceName("server")))
		})

		It("the rabbitmq image is updated", func() {
			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.Image = "rabbitmq:3.8.0"
			})).To(Succeed())

			Eventually(func() string {
				stsName := cluster.ChildResourceName("server")
				sts, _ := clientSet.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, stsName, metav1.GetOptions{})
				return sts.Spec.Template.Spec.Containers[0].Image
			}, 3).Should(Equal("rabbitmq:3.8.0"))
		})

		It("the rabbitmq ImagePullSecret is updated", func() {
			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.ImagePullSecret = "my-new-secret"
			})).To(Succeed())

			Eventually(func() []corev1.LocalObjectReference {
				stsName := cluster.ChildResourceName("server")
				sts, _ := clientSet.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, stsName, metav1.GetOptions{})
				Expect(len(sts.Spec.Template.Spec.ImagePullSecrets)).To(Equal(1))
				return sts.Spec.Template.Spec.ImagePullSecrets
			}, 3).Should(ConsistOf(corev1.LocalObjectReference{Name: "my-new-secret"}))
		})

		It("labels are updated", func() {
			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Labels = make(map[string]string)
				r.Labels["foo"] = "bar"
			})).To(Succeed())

			Eventually(func() map[string]string {
				service, err := clientSet.CoreV1().Services(cluster.Namespace).Get(ctx, clientServiceName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				return service.Labels
			}, 3).Should(HaveKeyWithValue("foo", "bar"))
			var sts *appsv1.StatefulSet
			Eventually(func() map[string]string {
				sts, _ = clientSet.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, statefulSetName, metav1.GetOptions{})
				return sts.Labels
			}, 3).Should(HaveKeyWithValue("foo", "bar"))
		})

		When("instance annotations are updated", func() {
			annotationKey := "anno-key"
			annotationValue := "anno-value"

			BeforeEach(func() {
				Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
					r.Annotations = make(map[string]string)
					r.Annotations[annotationKey] = annotationValue
				})).To(Succeed())
			})

			It("updates annotations for services", func() {
				Eventually(func() map[string]string {
					service, err := clientSet.CoreV1().Services(cluster.Namespace).Get(ctx, cluster.ChildResourceName("headless"), metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					return service.Annotations
				}, 3).Should(HaveKeyWithValue(annotationKey, annotationValue))

				Eventually(func() map[string]string {
					service, err := clientSet.CoreV1().Services(cluster.Namespace).Get(ctx, cluster.ChildResourceName("client"), metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					return service.Annotations
				}, 3).Should(HaveKeyWithValue(annotationKey, annotationValue))
			})

			It("updates annotations for stateful set", func() {
				Eventually(func() map[string]string {
					sts, err := clientSet.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, statefulSetName, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					return sts.Annotations
				}, 3).Should(HaveKeyWithValue(annotationKey, annotationValue))
			})

			It("updates annotations for role binding", func() {
				Eventually(func() map[string]string {
					roleBinding, err := clientSet.RbacV1().RoleBindings(cluster.Namespace).Get(ctx, cluster.ChildResourceName("server"), metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					return roleBinding.Annotations
				}, 3).Should(HaveKeyWithValue(annotationKey, annotationValue))
			})

			It("updates annotations for role", func() {
				Eventually(func() map[string]string {
					roleBinding, err := clientSet.RbacV1().Roles(cluster.Namespace).Get(ctx, cluster.ChildResourceName("peer-discovery"), metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					return roleBinding.Annotations
				}, 3).Should(HaveKeyWithValue(annotationKey, annotationValue))
			})

			It("updates annotations for service account", func() {
				Eventually(func() map[string]string {
					roleBinding, err := clientSet.CoreV1().ServiceAccounts(cluster.Namespace).Get(ctx, cluster.ChildResourceName("server"), metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					return roleBinding.Annotations
				}, 3).Should(HaveKeyWithValue(annotationKey, annotationValue))
			})

			It("updates annotations for secrets", func() {
				Eventually(func() map[string]string {
					roleBinding, err := clientSet.CoreV1().Secrets(cluster.Namespace).Get(ctx, cluster.ChildResourceName("admin"), metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					return roleBinding.Annotations
				}, 3).Should(HaveKeyWithValue(annotationKey, annotationValue))

				Eventually(func() map[string]string {
					roleBinding, err := clientSet.CoreV1().Secrets(cluster.Namespace).Get(ctx, cluster.ChildResourceName("erlang-cookie"), metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					return roleBinding.Annotations
				}, 3).Should(HaveKeyWithValue(annotationKey, annotationValue))
			})

			It("updates annotations for config maps", func() {
				Eventually(func() map[string]string {
					roleBinding, err := clientSet.CoreV1().ConfigMaps(cluster.Namespace).Get(ctx, cluster.ChildResourceName("server-conf"), metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					return roleBinding.Annotations
				}, 3).Should(HaveKeyWithValue(annotationKey, annotationValue))

				Eventually(func() map[string]string {
					roleBinding, err := clientSet.CoreV1().ConfigMaps(cluster.Namespace).Get(ctx, cluster.ChildResourceName("plugins-conf"), metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					return roleBinding.Annotations
				}, 3).Should(HaveKeyWithValue(annotationKey, annotationValue))
			})

			It("updates annotations for config maps", func() {
				Eventually(func() map[string]string {
					roleBinding, err := clientSet.CoreV1().ConfigMaps(cluster.Namespace).Get(ctx, cluster.ChildResourceName("server-conf"), metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					return roleBinding.Annotations
				}, 3).Should(HaveKeyWithValue(annotationKey, annotationValue))

				Eventually(func() map[string]string {
					roleBinding, err := clientSet.CoreV1().ConfigMaps(cluster.Namespace).Get(ctx, cluster.ChildResourceName("plugins-conf"), metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					return roleBinding.Annotations
				}, 3).Should(HaveKeyWithValue(annotationKey, annotationValue))
			})

		})

		It("service type is updated", func() {
			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.Service.Type = "NodePort"
			})).To(Succeed())

			Eventually(func() string {
				service, err := clientSet.CoreV1().Services(cluster.Namespace).Get(ctx, cluster.ChildResourceName("client"), metav1.GetOptions{})
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

			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.Affinity = affinity
			})).To(Succeed())

			Eventually(func() *corev1.Affinity {
				sts, _ := clientSet.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, statefulSetName, metav1.GetOptions{})
				return sts.Spec.Template.Spec.Affinity
			}, 3).Should(Equal(affinity))

			Expect(client.Get(
				ctx,
				types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace},
				cluster,
			)).To(Succeed())

			affinity = nil
			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.Affinity = affinity
			})).To(Succeed())
			Eventually(func() *corev1.Affinity {
				sts, _ := clientSet.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, statefulSetName, metav1.GetOptions{})
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
		)
		BeforeEach(func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-delete",
					Namespace: defaultNamespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas:        &one,
					ImagePullSecret: "rabbit-two-secret",
				},
			}
			clientServiceName = cluster.ChildResourceName("client")
			headlessServiceName = cluster.ChildResourceName("headless")
			stsName = cluster.ChildResourceName("server")
			configMapName = cluster.ChildResourceName("server-conf")

			Expect(client.Create(ctx, cluster)).To(Succeed())
			time.Sleep(500 * time.Millisecond)
		})

		AfterEach(func() {
			Expect(client.Delete(ctx, cluster)).To(Succeed())
		})

		It("recreates child resources after deletion", func() {
			oldConfMap, err := clientSet.CoreV1().ConfigMaps(defaultNamespace).Get(ctx, configMapName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			oldClientSvc := service(ctx, cluster, "client")

			oldHeadlessSvc := service(ctx, cluster, "headless")

			oldSts := statefulSet(ctx, cluster)

			Expect(clientSet.AppsV1().StatefulSets(defaultNamespace).Delete(ctx, stsName, metav1.DeleteOptions{})).NotTo(HaveOccurred())
			Expect(clientSet.CoreV1().ConfigMaps(defaultNamespace).Delete(ctx, configMapName, metav1.DeleteOptions{})).NotTo(HaveOccurred())
			Expect(clientSet.CoreV1().Services(defaultNamespace).Delete(ctx, clientServiceName, metav1.DeleteOptions{})).NotTo(HaveOccurred())
			Expect(clientSet.CoreV1().Services(defaultNamespace).Delete(ctx, headlessServiceName, metav1.DeleteOptions{})).NotTo(HaveOccurred())

			Eventually(func() bool {
				sts, err := clientSet.AppsV1().StatefulSets(defaultNamespace).Get(ctx, stsName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return string(sts.UID) != string(oldSts.UID)
			}, 5).Should(BeTrue())

			Eventually(func() bool {
				clientSvc, err := clientSet.CoreV1().Services(defaultNamespace).Get(ctx, clientServiceName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return string(clientSvc.UID) != string(oldClientSvc.UID)
			}, 5).Should(BeTrue())

			Eventually(func() bool {
				headlessSvc, err := clientSet.CoreV1().Services(defaultNamespace).Get(ctx, headlessServiceName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return string(headlessSvc.UID) != string(oldHeadlessSvc.UID)
			}, 5).Should(Not(Equal(oldHeadlessSvc.UID)))

			Eventually(func() bool {
				configMap, err := clientSet.CoreV1().ConfigMaps(defaultNamespace).Get(ctx, configMapName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				return string(configMap.UID) != string(oldConfMap.UID)
			}, 5).Should(Not(Equal(oldConfMap.UID)))

		})
	})

	Context("RabbitMQ CR ReconcileSuccess condition", func() {
		var crName string

		BeforeEach(func() {
			crName = "irreconcilable"
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      crName,
					Namespace: defaultNamespace,
				},
			}
			cluster.Spec.Replicas = &one
		})

		It("exposes ReconcileSuccess condition", func() {
			By("setting to False when spec is not valid", func() {
				// Annotations must end in alphanumeric character. However KubeAPI will accept this manifest
				cluster.Spec.Service.Annotations = map[string]string{"thisIs-": "notValidForK8s"}
				Expect(client.Create(ctx, cluster)).To(Succeed())
				waitForClusterCreation(ctx, cluster, client)

				Eventually(func() string {
					someRabbit := &rabbitmqv1beta1.RabbitmqCluster{}
					Expect(client.Get(ctx, runtimeClient.ObjectKey{
						Name:      crName,
						Namespace: defaultNamespace,
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
				Expect(client.Get(ctx, runtimeClient.ObjectKey{
					Name:      crName,
					Namespace: defaultNamespace,
				}, cluster)).To(Succeed())
				cluster.Spec.Service.Annotations = map[string]string{"thisIs": "valid"}
				Expect(client.Update(ctx, cluster)).To(Succeed())

				Eventually(func() string {
					someRabbit := &rabbitmqv1beta1.RabbitmqCluster{}
					Expect(client.Get(ctx, runtimeClient.ObjectKey{
						Name:      crName,
						Namespace: defaultNamespace,
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
			q, myStorage     k8sresource.Quantity
			storageClassName string
		)

		BeforeEach(func() {
			storageClassName = "my-storage-class"
			myStorage = k8sresource.MustParse("100Gi")
			q, _ = k8sresource.ParseQuantity("10Gi")
			ten := int32(10)
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-sts-override",
					Namespace: defaultNamespace,
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
											Namespace: defaultNamespace,
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
											Namespace: defaultNamespace,
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
			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)
		})

		AfterEach(func() {
			Expect(client.Delete(ctx, cluster)).To(Succeed())
			waitForClusterDeletion(ctx, cluster, client)
		})

		It("creates a StatefulSet with the override applied", func() {
			sts := statefulSet(ctx, cluster)
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
			Expect(sts.Spec.VolumeClaimTemplates[0].ObjectMeta.Namespace).To(Equal("default"))
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
			Expect(sts.Spec.VolumeClaimTemplates[1].ObjectMeta.Namespace).To(Equal("default"))
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
					Name: "rabbitmq-confd",
					VolumeSource: corev1.VolumeSource{
						Projected: &corev1.ProjectedVolumeSource{
							Sources: []corev1.VolumeProjection{
								{
									Secret: &corev1.SecretProjection{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "rabbitmq-sts-override-rabbitmq-admin",
										},
										Items: []corev1.KeyToPath{
											{
												Key:  "default_user.conf",
												Path: "default_user.conf",
											},
										},
									},
								},
							},
							DefaultMode: &defaultMode,
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
					Name: "plugins-conf",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							DefaultMode: &defaultMode,
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "rabbitmq-sts-override-rabbitmq-plugins-conf",
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

			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				cluster.Spec.Override.StatefulSet.Spec.Replicas = &five
				cluster.Spec.Override.StatefulSet.Spec.Template.Spec.Containers = []corev1.Container{
					{
						Name:  "additional-container-2",
						Image: "my-great-image-2",
					},
				}
			})).To(Succeed())

			Eventually(func() int32 {
				sts := statefulSet(ctx, cluster)
				return *sts.Spec.Replicas
			}, 3).Should(Equal(int32(5)))

			Eventually(func() string {
				sts := statefulSet(ctx, cluster)
				c := extractContainer(sts.Spec.Template.Spec.Containers, "additional-container-2")
				return c.Image
			}, 3).Should(Equal("my-great-image-2"))
		})
	})

	Context("Client Service Override", func() {

		BeforeEach(func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "svc-override",
					Namespace: defaultNamespace,
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

			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)
		})

		AfterEach(func() {
			Expect(client.Delete(ctx, cluster)).To(Succeed())
			waitForClusterDeletion(ctx, cluster, client)
		})

		It("creates a Client Service with the override applied", func() {
			amqpTargetPort := intstr.IntOrString{IntVal: int32(5672)}
			managementTargetPort := intstr.IntOrString{IntVal: int32(15672)}
			additionalTargetPort := intstr.IntOrString{IntVal: int32(15535)}
			svc := service(ctx, cluster, "client")
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
			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				cluster.Spec.Override.ClientService.Spec.Type = "LoadBalancer"
			})).To(Succeed())

			Eventually(func() corev1.ServiceType {
				svc := service(ctx, cluster, "client")
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
func aggregateEventMsgs(ctx context.Context, rabbit *rabbitmqv1beta1.RabbitmqCluster, reason string) string {
	events, err := clientSet.CoreV1().Events(rabbit.Namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.namespace=%s,reason=%s", rabbit.Name, rabbit.Namespace, reason),
	})
	ExpectWithOffset(1, err).To(Succeed())
	var msgs string
	for _, e := range events.Items {
		msgs = msgs + e.Message + " "
	}
	return msgs
}

func statefulSet(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) *appsv1.StatefulSet {
	stsName := rabbitmqCluster.ChildResourceName("server")
	var sts *appsv1.StatefulSet
	EventuallyWithOffset(1, func() error {
		var err error
		sts, err = clientSet.AppsV1().StatefulSets(rabbitmqCluster.Namespace).Get(ctx, stsName, metav1.GetOptions{})
		return err
	}, 10).Should(Succeed())
	return sts
}

func service(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster, svcName string) *corev1.Service {
	serviceName := rabbitmqCluster.ChildResourceName(svcName)
	var svc *corev1.Service
	EventuallyWithOffset(1, func() error {
		var err error
		svc, err = clientSet.CoreV1().Services(rabbitmqCluster.Namespace).Get(ctx, serviceName, metav1.GetOptions{})
		return err
	}, 10).Should(Succeed())
	return svc
}

func tlsSecretWithCACert(ctx context.Context, secretName, namespace, caCertName string) {
	tlsData := map[string]string{
		"tls.crt":  "this is a tls cert",
		"tls.key":  "this is a tls key",
		caCertName: "certificate",
	}

	_, err := createSecret(ctx, secretName, namespace, tlsData)

	if !apierrors.IsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred())
	}
}

func tlsSecretWithoutCACert(ctx context.Context, secretName, namespace string) {
	tlsData := map[string]string{
		"tls.crt": "this is a tls cert",
		"tls.key": "this is a tls key",
	}
	_, err := createSecret(ctx, secretName, namespace, tlsData)

	if !apierrors.IsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred())
	}
}

func createSecret(ctx context.Context, secretName string, namespace string, data map[string]string) (corev1.Secret, error) {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		StringData: data,
	}

	_, err := clientSet.CoreV1().Secrets(namespace).Create(ctx, &secret, metav1.CreateOptions{})
	return secret, err
}

func rabbitmqClusterWithTLS(ctx context.Context, clustername string, namespace string, tlsSpec rabbitmqv1beta1.TLSSpec) *rabbitmqv1beta1.RabbitmqCluster {
	var one int32 = 1
	rabbitmqCluster := &rabbitmqv1beta1.RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clustername,
			Namespace: namespace,
		},
		Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
			Replicas: &one,
			TLS:      tlsSpec,
		},
	}

	Expect(client.Create(ctx, rabbitmqCluster)).To(Succeed())

	return rabbitmqCluster
}

func waitForClusterCreation(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster, client runtimeClient.Client) {
	EventuallyWithOffset(1, func() string {
		rabbitmqClusterCreated := rabbitmqv1beta1.RabbitmqCluster{}
		err := client.Get(
			ctx,
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

func waitForClusterDeletion(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster, client runtimeClient.Client) {
	EventuallyWithOffset(1, func() bool {
		rabbitmqClusterCreated := rabbitmqv1beta1.RabbitmqCluster{}
		err := client.Get(
			ctx,
			types.NamespacedName{Name: rabbitmqCluster.Name, Namespace: rabbitmqCluster.Namespace},
			&rabbitmqClusterCreated,
		)
		return apierrors.IsNotFound(err)
	}, ClusterDeletionTimeout, 1*time.Second).Should(BeTrue())

}

func verifyError(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster, expectedError string) {
	tlsEventTimeout := 5 * time.Second
	tlsRetry := 1 * time.Second
	Eventually(func() string { return aggregateEventMsgs(ctx, rabbitmqCluster, "TLSError") }, tlsEventTimeout, tlsRetry).Should(ContainSubstring(expectedError))
}
