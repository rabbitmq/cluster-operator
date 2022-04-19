// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.

package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rabbitmq/cluster-operator/internal/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	"golang.org/x/net/context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("RabbitmqCluster", func() {

	Context("RabbitmqClusterSpec", func() {
		It("can be created with a single replica", func() {
			created := generateRabbitmqClusterObject("rabbit1")
			created.Spec.Replicas = pointer.Int32Ptr(1)
			Expect(k8sClient.Create(context.Background(), created)).To(Succeed())

			fetched := &RabbitmqCluster{}
			Expect(k8sClient.Get(context.Background(), getKey(created), fetched)).To(Succeed())
			Expect(fetched).To(Equal(created))
		})

		It("can be created with three replicas", func() {
			created := generateRabbitmqClusterObject("rabbit2")
			created.Spec.Replicas = pointer.Int32Ptr(3)
			Expect(k8sClient.Create(context.Background(), created)).To(Succeed())

			fetched := &RabbitmqCluster{}
			Expect(k8sClient.Get(context.Background(), getKey(created), fetched)).To(Succeed())
			Expect(fetched).To(Equal(created))
		})

		It("can be created with five replicas", func() {
			created := generateRabbitmqClusterObject("rabbit3")
			created.Spec.Replicas = pointer.Int32Ptr(5)
			Expect(k8sClient.Create(context.Background(), created)).To(Succeed())

			fetched := &RabbitmqCluster{}
			Expect(k8sClient.Get(context.Background(), getKey(created), fetched)).To(Succeed())
			Expect(fetched).To(Equal(created))
		})

		It("can be deleted", func() {
			created := generateRabbitmqClusterObject("rabbit4")
			Expect(k8sClient.Create(context.Background(), created)).To(Succeed())

			Expect(k8sClient.Delete(context.Background(), created)).To(Succeed())
			Expect(k8sClient.Get(context.Background(), getKey(created), created)).ToNot(Succeed())
		})

		It("can be created with resource requests", func() {
			created := generateRabbitmqClusterObject("rabbit-resource-request")
			created.Spec.Resources = &corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    k8sresource.MustParse("100m"),
					corev1.ResourceMemory: k8sresource.MustParse("100Mi"),
				},
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    k8sresource.MustParse("100m"),
					corev1.ResourceMemory: k8sresource.MustParse("100Mi"),
				},
			}
			Expect(k8sClient.Create(context.Background(), created)).To(Succeed())
		})

		It("can be created with server side TLS", func() {
			created := generateRabbitmqClusterObject("rabbit-tls")
			created.Spec.TLS.SecretName = "tls-secret-name"
			Expect(k8sClient.Create(context.Background(), created)).To(Succeed())
		})

		It("can be queried if TLS is enabled", func() {
			created := generateRabbitmqClusterObject("rabbit-tls")
			Expect(created.TLSEnabled()).To(BeFalse())

			created.Spec.TLS.SecretName = "tls-secret-name"
			Expect(created.TLSEnabled()).To(BeTrue())
		})

		It("can be queried if mutual TLS is enabled", func() {
			created := generateRabbitmqClusterObject("rabbit-mutual-tls")
			Expect(created.MutualTLSEnabled()).To(BeFalse())

			created.Spec.TLS.SecretName = "tls-secret-name"
			Expect(created.MutualTLSEnabled()).To(BeFalse())

			created.Spec.TLS.SecretName = ""
			created.Spec.TLS.CaSecretName = "tls-secret-name"
			Expect(created.MutualTLSEnabled()).To(BeFalse())

			created.Spec.TLS.SecretName = "tls-secret-name"
			created.Spec.TLS.CaSecretName = "tls-secret-name"
			Expect(created.MutualTLSEnabled()).To(BeTrue())
		})

		It("can be queried if memory limits are provided", func() {
			created := generateRabbitmqClusterObject("rabbit-mem-limit")
			Expect(created.MemoryLimited()).To(BeTrue())

			created.Spec.Resources = &corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{},
			}
			Expect(created.MemoryLimited()).To(BeFalse())
		})

		It("is validated", func() {
			By("checking the replica count", func() {
				invalidReplica := generateRabbitmqClusterObject("rabbit4")
				invalidReplica.Spec.Replicas = pointer.Int32Ptr(-1)
				Expect(apierrors.IsInvalid(k8sClient.Create(context.Background(), invalidReplica))).To(BeTrue())
				Expect(k8sClient.Create(context.Background(), invalidReplica)).To(MatchError(ContainSubstring("spec.replicas in body should be greater than or equal to 0")))
			})

			By("checking the service type", func() {
				invalidService := generateRabbitmqClusterObject("rabbit5")
				invalidService.Spec.Service.Type = "ihateservices"
				Expect(apierrors.IsInvalid(k8sClient.Create(context.Background(), invalidService))).To(BeTrue())
				Expect(k8sClient.Create(context.Background(), invalidService)).To(MatchError(ContainSubstring("supported values: \"ClusterIP\", \"LoadBalancer\", \"NodePort\"")))
			})
		})

		Describe("ChildResourceName", func() {
			It("prefixes the passed string with the name of the RabbitmqCluster name", func() {
				resource := generateRabbitmqClusterObject("iam")
				Expect(resource.ChildResourceName("great")).To(Equal("iam-great"))
			})
		})

		Context("Default settings", func() {
			var (
				rmqClusterInstance      RabbitmqCluster
				expectedClusterInstance RabbitmqCluster
			)

			BeforeEach(func() {
				expectedClusterInstance = *generateRabbitmqClusterObject("foo")
			})

			When("CR spec is empty", func() {
				It("creates CR with defaults", func() {
					rmqClusterInstance = RabbitmqCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "rabbitmq-defaults",
							Namespace: "default",
						},
					}

					Expect(k8sClient.Create(context.Background(), &rmqClusterInstance)).To(Succeed())
					fetchedRabbit := &RabbitmqCluster{}
					Expect(k8sClient.Get(context.Background(), getKey(&rmqClusterInstance), fetchedRabbit)).To(Succeed())
					Expect(fetchedRabbit.Spec).To(Equal(expectedClusterInstance.Spec))
				})
			})

			When("CR is fully populated", func() {
				It("outputs the CR", func() {
					storage := k8sresource.MustParse("987Gi")
					storageClassName := "some-class"
					rmqClusterInstance = RabbitmqCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "rabbitmq-full-manifest",
							Namespace: "default",
						},
						Spec: RabbitmqClusterSpec{
							Replicas:                      pointer.Int32Ptr(3),
							Image:                         "rabbitmq-image-from-cr",
							ImagePullSecrets:              []corev1.LocalObjectReference{{Name: "my-super-secret"}},
							TerminationGracePeriodSeconds: pointer.Int64Ptr(0),
							Service: RabbitmqClusterServiceSpec{
								Type: "NodePort",
								Annotations: map[string]string{
									"myannotation": "is-set",
								},
							},
							Persistence: RabbitmqClusterPersistenceSpec{
								StorageClassName: &storageClassName,
								Storage:          &storage,
							},
							Resources: &corev1.ResourceRequirements{
								Limits: map[corev1.ResourceName]k8sresource.Quantity{
									"cpu":    k8sresource.MustParse("16"),
									"memory": k8sresource.MustParse("16Gi"),
								},
								Requests: map[corev1.ResourceName]k8sresource.Quantity{
									"cpu":    k8sresource.MustParse("15"),
									"memory": k8sresource.MustParse("15Gi"),
								},
							},
							Affinity: &corev1.Affinity{
								NodeAffinity: &corev1.NodeAffinity{
									RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
										NodeSelectorTerms: []corev1.NodeSelectorTerm{
											{
												MatchExpressions: []corev1.NodeSelectorRequirement{
													{
														Key:      "somekey",
														Operator: "Equal",
														Values:   []string{"this-value"},
													},
												},
												MatchFields: nil,
											},
										},
									},
								},
							},
							Tolerations: []corev1.Toleration{
								{
									Key:      "mykey",
									Operator: "NotEqual",
									Value:    "myvalue",
									Effect:   "NoSchedule",
								},
							},
							Rabbitmq: RabbitmqClusterConfigurationSpec{
								AdditionalPlugins: []Plugin{
									"my_plugins",
								},
							},
						},
					}

					Expect(k8sClient.Create(context.Background(), &rmqClusterInstance)).To(Succeed())
					fetchedRabbit := &RabbitmqCluster{}
					Expect(k8sClient.Get(context.Background(), getKey(&rmqClusterInstance), fetchedRabbit)).To(Succeed())
					Expect(fetchedRabbit.Spec).To(Equal(rmqClusterInstance.Spec))
				})
			})

			When("CR is partially set", func() {

				It("applies default values to missing properties if image is set", func() {
					rmqClusterInstance = RabbitmqCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "rabbitmq-image",
							Namespace: "default",
						},
						Spec: RabbitmqClusterSpec{
							Image: "test-image",
						},
					}

					expectedClusterInstance.Spec.Image = "test-image"

					Expect(k8sClient.Create(context.Background(), &rmqClusterInstance)).To(Succeed())
					fetchedRabbit := &RabbitmqCluster{}
					Expect(k8sClient.Get(context.Background(), getKey(&rmqClusterInstance), fetchedRabbit)).To(Succeed())
					Expect(fetchedRabbit.Spec).To(Equal(expectedClusterInstance.Spec))
				})

				It("does not apply resource defaults if the resource object is an empty non-nil struct", func() {
					expectedResources := &corev1.ResourceRequirements{}
					rmqClusterInstance = RabbitmqCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "rabbitmq-empty-resource",
							Namespace: "default",
						},
						Spec: RabbitmqClusterSpec{
							Resources: expectedResources,
						},
					}

					expectedClusterInstance.Spec.Resources = expectedResources

					Expect(k8sClient.Create(context.Background(), &rmqClusterInstance)).To(Succeed())
					fetchedRabbit := &RabbitmqCluster{}
					Expect(k8sClient.Get(context.Background(), getKey(&rmqClusterInstance), fetchedRabbit)).To(Succeed())
					Expect(fetchedRabbit.Spec).To(Equal(expectedClusterInstance.Spec))
				})

				It("does not apply resource defaults if the resource object is partially set", func() {
					expectedResources := &corev1.ResourceRequirements{
						Limits: map[corev1.ResourceName]k8sresource.Quantity{
							"cpu": k8sresource.MustParse("6"),
						},
					}
					rmqClusterInstance = RabbitmqCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "rabbitmq-partial-resource",
							Namespace: "default",
						},
						Spec: RabbitmqClusterSpec{
							Resources: expectedResources,
						},
					}

					expectedClusterInstance.Spec.Resources = expectedResources

					Expect(k8sClient.Create(context.Background(), &rmqClusterInstance)).To(Succeed())
					fetchedRabbit := &RabbitmqCluster{}
					Expect(k8sClient.Get(context.Background(), getKey(&rmqClusterInstance), fetchedRabbit)).To(Succeed())
					Expect(fetchedRabbit.Spec).To(Equal(expectedClusterInstance.Spec))
				})

				It("sets spec.service.type if spec.service is partially set", func() {
					rmqClusterInstance = RabbitmqCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "rabbit-service-type",
							Namespace: "default",
						},
						Spec: RabbitmqClusterSpec{
							Service: RabbitmqClusterServiceSpec{
								Annotations: map[string]string{"key": "value"},
							},
						},
					}

					expectedClusterInstance.Spec.Service = RabbitmqClusterServiceSpec{
						Annotations: map[string]string{"key": "value"},
						Type:        "ClusterIP",
					}

					Expect(k8sClient.Create(context.Background(), &rmqClusterInstance)).To(Succeed())
					fetchedRabbit := &RabbitmqCluster{}
					Expect(k8sClient.Get(context.Background(), getKey(&rmqClusterInstance), fetchedRabbit)).To(Succeed())
					Expect(fetchedRabbit.Spec).To(Equal(expectedClusterInstance.Spec))
				})

				It("sets spec.persistence.storage if spec.persistence is partially set", func() {
					myStorage := "mystorage"
					tenGi := k8sresource.MustParse("10Gi")
					rmqClusterInstance = RabbitmqCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "rabbit-storage",
							Namespace: "default",
						},
						Spec: RabbitmqClusterSpec{
							Persistence: RabbitmqClusterPersistenceSpec{
								StorageClassName: &myStorage,
							},
						},
					}

					expectedClusterInstance.Spec.Persistence = RabbitmqClusterPersistenceSpec{
						StorageClassName: &myStorage,
						Storage:          &tenGi,
					}

					Expect(k8sClient.Create(context.Background(), &rmqClusterInstance)).To(Succeed())
					fetchedRabbit := &RabbitmqCluster{}
					Expect(k8sClient.Get(context.Background(), getKey(&rmqClusterInstance), fetchedRabbit)).To(Succeed())
					Expect(fetchedRabbit.Spec).To(Equal(expectedClusterInstance.Spec))
				})
			})
		})
		Context("Vault", func() {
			It("is disabled by default", func() {
				rabbit := generateRabbitmqClusterObject("rabbit-without-vault")
				Expect(k8sClient.Create(context.Background(), rabbit)).To(Succeed())
				fetchedRabbit := &RabbitmqCluster{}
				Expect(k8sClient.Get(context.Background(), getKey(rabbit), fetchedRabbit)).To(Succeed())

				Expect(fetchedRabbit.VaultEnabled()).To(BeFalse())
			})
			When("only default user is configured", func() {
				It("sets vault configuration correctly", func() {
					rabbit := generateRabbitmqClusterObject("rabbit-vault-default-user")
					rabbit.Spec.SecretBackend.Vault = &VaultSpec{
						Role:            "test-role",
						DefaultUserPath: "test-path",
					}
					Expect(k8sClient.Create(context.Background(), rabbit)).To(Succeed())
					fetchedRabbit := &RabbitmqCluster{}
					Expect(k8sClient.Get(context.Background(), getKey(rabbit), fetchedRabbit)).To(Succeed())

					Expect(fetchedRabbit.Spec.SecretBackend.Vault.Role).To(Equal("test-role"))
					Expect(fetchedRabbit.Spec.SecretBackend.Vault.DefaultUserPath).To(Equal("test-path"))
					Expect(fetchedRabbit.VaultEnabled()).To(BeTrue())
					Expect(fetchedRabbit.VaultDefaultUserSecretEnabled()).To(BeTrue())
					Expect(fetchedRabbit.Spec.SecretBackend.Vault.DefaultUserSecretEnabled()).To(BeTrue())
					Expect(fetchedRabbit.VaultTLSEnabled()).To(BeFalse())
					Expect(fetchedRabbit.Spec.SecretBackend.Vault.TLSEnabled()).To(BeFalse())
				})
			})
			When("only TLS is configured", func() {
				It("sets vault configuration correctly", func() {
					rabbit := generateRabbitmqClusterObject("rabbit-vault-tls")
					rabbit.Spec.SecretBackend.Vault = &VaultSpec{
						Role: "test-role",
						TLS: VaultTLSSpec{
							PKIIssuerPath: "pki/issue/hashicorp-com",
						},
					}
					Expect(k8sClient.Create(context.Background(), rabbit)).To(Succeed())
					fetchedRabbit := &RabbitmqCluster{}
					Expect(k8sClient.Get(context.Background(), getKey(rabbit), fetchedRabbit)).To(Succeed())

					Expect(fetchedRabbit.Spec.SecretBackend.Vault.Role).To(Equal("test-role"))
					Expect(fetchedRabbit.Spec.SecretBackend.Vault.TLS.PKIIssuerPath).To(Equal("pki/issue/hashicorp-com"))
					Expect(fetchedRabbit.VaultEnabled()).To(BeTrue())
					Expect(fetchedRabbit.VaultDefaultUserSecretEnabled()).To(BeFalse())
					Expect(fetchedRabbit.Spec.SecretBackend.Vault.DefaultUserSecretEnabled()).To(BeFalse())
					Expect(fetchedRabbit.VaultTLSEnabled()).To(BeTrue())
					Expect(fetchedRabbit.Spec.SecretBackend.Vault.TLSEnabled()).To(BeTrue())
				})
			})
		})
	})
	Context("RabbitmqClusterStatus", func() {
		It("sets conditions based on inputs", func() {
			rabbitmqClusterStatus := RabbitmqClusterStatus{}
			statefulset := &appsv1.StatefulSet{}
			statefulset.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Limits: map[corev1.ResourceName]resource.Quantity{
							"memory": resource.MustParse("100Mi"),
						},
						Requests: map[corev1.ResourceName]resource.Quantity{
							"memory": resource.MustParse("100Mi"),
						},
					},
				},
			}

			statefulset.Status = appsv1.StatefulSetStatus{
				ObservedGeneration: 0,
				Replicas:           0,
				ReadyReplicas:      3,
				CurrentReplicas:    0,
				UpdatedReplicas:    0,
				CurrentRevision:    "",
				UpdateRevision:     "",
				CollisionCount:     nil,
				Conditions:         nil,
			}

			endPoints := &corev1.Endpoints{
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{
								IP: "127.0.0.1",
							},
						},
					},
				},
			}

			rabbitmqClusterStatus.SetConditions([]runtime.Object{statefulset, endPoints})

			Expect(rabbitmqClusterStatus.Conditions).To(HaveLen(4))
			Expect(rabbitmqClusterStatus.Conditions[0].Type).To(Equal(status.AllReplicasReady))
			Expect(rabbitmqClusterStatus.Conditions[1].Type).To(Equal(status.ClusterAvailable))
			Expect(rabbitmqClusterStatus.Conditions[2].Type).To(Equal(status.NoWarnings))
			Expect(rabbitmqClusterStatus.Conditions[3].Type).To(Equal(status.ReconcileSuccess))
		})

		It("updates an arbitrary condition", func() {
			someCondition := status.RabbitmqClusterCondition{}
			someCondition.Type = "a-type"
			someCondition.Reason = "whynot"
			someCondition.Status = "perhaps"
			someCondition.LastTransitionTime = metav1.Unix(10, 0)
			rmqStatus := RabbitmqClusterStatus{
				Conditions: []status.RabbitmqClusterCondition{someCondition},
			}

			rmqStatus.SetCondition("a-type",
				corev1.ConditionTrue, "some-reason", "my-message")

			updatedCondition := rmqStatus.Conditions[0]
			Expect(updatedCondition.Status).To(Equal(corev1.ConditionTrue))
			Expect(updatedCondition.Reason).To(Equal("some-reason"))
			Expect(updatedCondition.Message).To(Equal("my-message"))

			notExpectedTime := metav1.Unix(10, 0)
			Expect(updatedCondition.LastTransitionTime).NotTo(Equal(notExpectedTime))
			Expect(updatedCondition.LastTransitionTime.Before(&notExpectedTime)).To(BeFalse())
		})
	})
	Context("PVC Name helper function", func() {
		It("returns the correct PVC name", func() {
			r := generateRabbitmqClusterObject("testrabbit")
			Expect(r.PVCName(0)).To(Equal("persistence-testrabbit-server-0"))
		})
	})
})

func getKey(cluster *RabbitmqCluster) types.NamespacedName {
	return types.NamespacedName{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}
}

func generateRabbitmqClusterObject(clusterName string) *RabbitmqCluster {
	storage := k8sresource.MustParse("10Gi")
	return &RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: "default",
		},
		Spec: RabbitmqClusterSpec{
			Replicas:                      pointer.Int32Ptr(1),
			TerminationGracePeriodSeconds: pointer.Int64Ptr(604800),
			Service: RabbitmqClusterServiceSpec{
				Type: "ClusterIP",
			},
			Resources: &corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]k8sresource.Quantity{
					"cpu":    k8sresource.MustParse("1000m"),
					"memory": k8sresource.MustParse("2Gi"),
				},
				Limits: map[corev1.ResourceName]k8sresource.Quantity{
					"cpu":    k8sresource.MustParse("2000m"),
					"memory": k8sresource.MustParse("2Gi"),
				},
			},
			Persistence: RabbitmqClusterPersistenceSpec{
				Storage: &storage,
			},
		},
	}
}
