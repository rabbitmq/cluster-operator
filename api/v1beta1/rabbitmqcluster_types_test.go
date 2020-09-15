// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.

package v1beta1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rabbitmq/cluster-operator/internal/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"

	"golang.org/x/net/context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("RabbitmqCluster", func() {

	var three int32 = 3

	Context("RabbitmqClusterSpec", func() {
		It("can be created with a single replica", func() {
			created := generateRabbitmqClusterObject("rabbit1")
			Expect(k8sClient.Create(context.TODO(), created)).To(Succeed())

			fetched := &RabbitmqCluster{}
			Expect(k8sClient.Get(context.TODO(), getKey(created), fetched)).To(Succeed())
			Expect(fetched).To(Equal(created))
		})

		It("can be created with three replicas", func() {
			created := generateRabbitmqClusterObject("rabbit2")
			created.Spec.Replicas = &three
			Expect(k8sClient.Create(context.TODO(), created)).To(Succeed())

			fetched := &RabbitmqCluster{}
			Expect(k8sClient.Get(context.TODO(), getKey(created), fetched)).To(Succeed())
			Expect(fetched).To(Equal(created))
		})

		It("can be created with five replicas", func() {
			five := int32(5)
			created := generateRabbitmqClusterObject("rabbit3")
			created.Spec.Replicas = &five
			Expect(k8sClient.Create(context.TODO(), created)).To(Succeed())

			fetched := &RabbitmqCluster{}
			Expect(k8sClient.Get(context.TODO(), getKey(created), fetched)).To(Succeed())
			Expect(fetched).To(Equal(created))
		})

		It("can be deleted", func() {
			created := generateRabbitmqClusterObject("rabbit4")
			Expect(k8sClient.Create(context.TODO(), created)).To(Succeed())

			Expect(k8sClient.Delete(context.TODO(), created)).To(Succeed())
			Expect(k8sClient.Get(context.TODO(), getKey(created), created)).ToNot(Succeed())
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
			Expect(k8sClient.Create(context.TODO(), created)).To(Succeed())
		})

		It("can be created with server side TLS", func() {
			created := generateRabbitmqClusterObject("rabbit-tls")
			created.Spec.TLS.SecretName = "tls-secret-name"
			Expect(k8sClient.Create(context.TODO(), created)).To(Succeed())
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

		It("is validated", func() {
			By("checking the replica count", func() {
				nOne := int32(-1)
				invalidReplica := generateRabbitmqClusterObject("rabbit4")
				invalidReplica.Spec.Replicas = &nOne
				Expect(apierrors.IsInvalid(k8sClient.Create(context.TODO(), invalidReplica))).To(BeTrue())
				Expect(k8sClient.Create(context.TODO(), invalidReplica)).To(MatchError(ContainSubstring("spec.replicas in body should be greater than or equal to 0")))
			})

			By("checking the service type", func() {
				invalidService := generateRabbitmqClusterObject("rabbit5")
				invalidService.Spec.Service.Type = "ihateservices"
				Expect(apierrors.IsInvalid(k8sClient.Create(context.TODO(), invalidService))).To(BeTrue())
				Expect(k8sClient.Create(context.TODO(), invalidService)).To(MatchError(ContainSubstring("supported values: \"ClusterIP\", \"LoadBalancer\", \"NodePort\"")))
			})
		})

		Describe("ChildResourceName", func() {
			It("prefixes the passed string with the name of the RabbitmqCluster name", func() {
				resource := generateRabbitmqClusterObject("iam")
				Expect(resource.ChildResourceName("great")).To(Equal("iam-rabbitmq-great"))
			})
		})

		Context("Default settings", func() {
			var (
				rmqClusterInstance RabbitmqCluster
				rmqClusterTemplate RabbitmqCluster
			)
			BeforeEach(func() {
				rmqClusterInstance = RabbitmqCluster{}
				rmqClusterTemplate = *generateRabbitmqClusterObject("foo")

			})

			When("CR is empty", func() {
				It("outputs the template", func() {
					instance := MergeDefaults(rmqClusterInstance)
					Expect(instance.Spec).To(Equal(rmqClusterTemplate.Spec))
				})
			})

			When("CR is fully populated", func() {
				It("outputs the CR", func() {
					storage := k8sresource.MustParse("987Gi")
					storageClassName := "some-class"
					rmqClusterInstance.Spec = RabbitmqClusterSpec{
						Replicas:        &three,
						Image:           "rabbitmq-image-from-cr",
						ImagePullSecret: "my-super-secret",
						Service: RabbitmqClusterServiceSpec{
							Type: "this-is-a-service",
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
								"my-plugins",
							},
						},
					}
					instance := MergeDefaults(rmqClusterInstance)
					Expect(instance.Spec).To(Equal(rmqClusterInstance.Spec))
				})
			})

			When("CR is partially set", func() {
				It("applies default values to missing properties if replicas is set", func() {
					rmqClusterInstance.Spec = RabbitmqClusterSpec{
						Replicas: &three,
					}
					expectedClusterInstance := rmqClusterTemplate.DeepCopy()
					expectedClusterInstance.Spec.Replicas = &three

					instance := MergeDefaults(rmqClusterInstance)
					Expect(instance.Spec).To(Equal(expectedClusterInstance.Spec))
				})

				It("applies default values to missing properties if image is set", func() {
					rmqClusterInstance.Spec = RabbitmqClusterSpec{
						Image: "test-image",
					}
					expectedClusterInstance := rmqClusterTemplate.DeepCopy()
					expectedClusterInstance.Spec.Image = "test-image"

					instance := MergeDefaults(rmqClusterInstance)
					Expect(instance.Spec).To(Equal(expectedClusterInstance.Spec))
				})

				It("does not apply resource defaults if the resource object is an empty non-nil struct", func() {
					expectedResources := &corev1.ResourceRequirements{}
					rmqClusterInstance.Spec = RabbitmqClusterSpec{
						Resources: expectedResources,
					}
					expectedClusterInstance := rmqClusterTemplate.DeepCopy()
					expectedClusterInstance.Spec.Resources = expectedResources

					instance := MergeDefaults(rmqClusterInstance)
					Expect(instance.Spec).To(Equal(expectedClusterInstance.Spec))

				})

				It("does not apply resource defaults if the resource object is partially set", func() {
					expectedResources := &corev1.ResourceRequirements{
						Limits: map[corev1.ResourceName]k8sresource.Quantity{
							"cpu": k8sresource.MustParse("6"),
						},
					}
					rmqClusterInstance.Spec = RabbitmqClusterSpec{
						Resources: expectedResources,
					}
					expectedClusterInstance := rmqClusterTemplate.DeepCopy()
					expectedClusterInstance.Spec.Resources = expectedResources

					instance := MergeDefaults(rmqClusterInstance)
					Expect(instance.Spec).To(Equal(expectedClusterInstance.Spec))
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
})

func getKey(cluster *RabbitmqCluster) types.NamespacedName {
	return types.NamespacedName{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}
}

func generateRabbitmqClusterObject(clusterName string) *RabbitmqCluster {
	storage := k8sresource.MustParse("10Gi")
	one := int32(1)
	return &RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: "default",
		},
		Spec: RabbitmqClusterSpec{
			Replicas: &one,
			Image:    "rabbitmq:3.8.8",
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
