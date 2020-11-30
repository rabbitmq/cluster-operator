// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
//

package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/resource"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
)

var _ = Describe("StatefulSet", func() {
	var (
		instance   rabbitmqv1beta1.RabbitmqCluster
		scheme     *runtime.Scheme
		builder    *resource.RabbitmqResourceBuilder
		stsBuilder *resource.StatefulSetBuilder
	)

	Describe("Build", func() {
		BeforeEach(func() {
			instance = generateRabbitmqCluster()

			scheme = runtime.NewScheme()
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
			Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())
			builder = &resource.RabbitmqResourceBuilder{
				Instance: &instance,
				Scheme:   scheme,
			}
			stsBuilder = builder.StatefulSet()
		})

		It("sets the name and namespace", func() {
			obj, err := stsBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			sts := obj.(*appsv1.StatefulSet)

			Expect(sts.Name).To(Equal("foo-server"))
			Expect(sts.Namespace).To(Equal("foo-namespace"))
		})

		It("sets the right service name", func() {
			obj, err := stsBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			statefulSet := obj.(*appsv1.StatefulSet)

			Expect(statefulSet.Spec.ServiceName).To(Equal(instance.ChildResourceName("nodes")))
		})
		It("adds the correct label selector", func() {
			obj, err := stsBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			statefulSet := obj.(*appsv1.StatefulSet)

			labels := statefulSet.Spec.Selector.MatchLabels
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
		})

		It("references the storageclassname when specified", func() {
			storageClassName := "my-storage-class"
			builder.Instance.Spec.Persistence.StorageClassName = &storageClassName

			obj, err := stsBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			statefulSet := obj.(*appsv1.StatefulSet)

			Expect(*statefulSet.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(Equal("my-storage-class"))
		})

		It("creates the PersistentVolume template according to configurations in the instance", func() {
			storage := k8sresource.MustParse("21Gi")
			builder.Instance.Spec.Persistence.Storage = &storage

			obj, err := stsBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			statefulSet := obj.(*appsv1.StatefulSet)

			q, _ := k8sresource.ParseQuantity("21Gi")
			Expect(statefulSet.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests["storage"]).To(Equal(q))
		})
		Context("PVC template", func() {
			It("creates the required PersistentVolumeClaim", func() {
				q, _ := k8sresource.ParseQuantity("10Gi")

				obj, err := stsBuilder.Build()
				Expect(err).NotTo(HaveOccurred())
				statefulSet := obj.(*appsv1.StatefulSet)

				expectedPersistentVolumeClaim := corev1.PersistentVolumeClaim{
					ObjectMeta: v1.ObjectMeta{
						Name:      "persistence",
						Namespace: instance.Namespace,
						Labels: map[string]string{
							"app.kubernetes.io/name":      instance.Name,
							"app.kubernetes.io/component": "rabbitmq",
							"app.kubernetes.io/part-of":   "rabbitmq",
						},
						OwnerReferences: []v1.OwnerReference{
							{
								APIVersion:         "rabbitmq.com/v1beta1",
								Kind:               "RabbitmqCluster",
								Name:               instance.Name,
								UID:                "",
								Controller:         pointer.BoolPtr(true),
								BlockOwnerDeletion: pointer.BoolPtr(false),
							},
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
				}

				actualPersistentVolumeClaim := statefulSet.Spec.VolumeClaimTemplates[0]
				Expect(actualPersistentVolumeClaim).To(Equal(expectedPersistentVolumeClaim))
			})
		})
		Context("Override", func() {
			It("overrides statefulSet.spec.selector", func() {
				builder.Instance.Spec.Override.StatefulSet = &rabbitmqv1beta1.StatefulSet{
					Spec: &rabbitmqv1beta1.StatefulSetSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"my-label": "my-label",
							},
						},
					},
				}

				obj, err := stsBuilder.Build()
				Expect(err).NotTo(HaveOccurred())
				statefulSet := obj.(*appsv1.StatefulSet)
				Expect(statefulSet.Spec.Selector.MatchLabels).To(Equal(map[string]string{"my-label": "my-label"}))
			})

			It("overrides statefulSet.spec.serviceName", func() {
				builder.Instance.Spec.Override.StatefulSet = &rabbitmqv1beta1.StatefulSet{
					Spec: &rabbitmqv1beta1.StatefulSetSpec{
						ServiceName: "mysevice",
					},
				}

				obj, err := stsBuilder.Build()
				Expect(err).NotTo(HaveOccurred())
				statefulSet := obj.(*appsv1.StatefulSet)
				Expect(statefulSet.Spec.ServiceName).To(Equal("mysevice"))
			})

			It("overrides the PVC list", func() {
				storageClass := "my-storage-class"
				builder.Instance.Spec.Override.StatefulSet = &rabbitmqv1beta1.StatefulSet{
					Spec: &rabbitmqv1beta1.StatefulSetSpec{
						VolumeClaimTemplates: []rabbitmqv1beta1.PersistentVolumeClaim{
							{
								EmbeddedObjectMeta: rabbitmqv1beta1.EmbeddedObjectMeta{
									Name:      "pert-1",
									Namespace: instance.Namespace,
								},
								Spec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: *instance.Spec.Persistence.Storage,
										},
									},
									StorageClassName: &storageClass,
								},
							},
							{
								EmbeddedObjectMeta: rabbitmqv1beta1.EmbeddedObjectMeta{
									Name:      "pert-2",
									Namespace: instance.Namespace,
								},
								Spec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: *instance.Spec.Persistence.Storage,
										},
									},
									StorageClassName: &storageClass,
								},
							},
						},
					},
				}
				stsBuilder := builder.StatefulSet()
				obj, err := stsBuilder.Build()
				Expect(err).NotTo(HaveOccurred())
				statefulSet := obj.(*appsv1.StatefulSet)

				Expect(statefulSet.Spec.VolumeClaimTemplates).To(ConsistOf(
					corev1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pert-1",
							Namespace: "foo-namespace",
							OwnerReferences: []v1.OwnerReference{
								{
									APIVersion:         "rabbitmq.com/v1beta1",
									Kind:               "RabbitmqCluster",
									Name:               instance.Name,
									UID:                "",
									Controller:         pointer.BoolPtr(true),
									BlockOwnerDeletion: pointer.BoolPtr(false),
								},
							},
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: *instance.Spec.Persistence.Storage,
								},
							},
							StorageClassName: &storageClass,
						},
					},
					corev1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pert-2",
							Namespace: "foo-namespace",
							OwnerReferences: []v1.OwnerReference{
								{
									APIVersion:         "rabbitmq.com/v1beta1",
									Kind:               "RabbitmqCluster",
									Name:               instance.Name,
									UID:                "",
									Controller:         pointer.BoolPtr(true),
									BlockOwnerDeletion: pointer.BoolPtr(false),
								},
							},
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: *instance.Spec.Persistence.Storage,
								},
							},
							StorageClassName: &storageClass,
						},
					},
				))
			})
		})
	})

	Describe("Update", func() {
		var (
			statefulSet *appsv1.StatefulSet
			stsBuilder  *resource.StatefulSetBuilder
		)

		BeforeEach(func() {
			instance = generateRabbitmqCluster()
			scheme = runtime.NewScheme()
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
			Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())

			builder = &resource.RabbitmqResourceBuilder{
				Instance: &instance,
				Scheme:   scheme,
			}

			stsBuilder = builder.StatefulSet()

			statefulSet = &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance.Name,
					Namespace: instance.Namespace,
				},
			}
		})

		It("creates the affinity rule as provided in the instance", func() {
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
			stsBuilder.Instance.Spec.Affinity = affinity

			Expect(stsBuilder.Update(statefulSet)).To(Succeed())
			Expect(statefulSet.Spec.Template.Spec.Affinity).To(Equal(affinity))
		})

		It("sets the owner reference", func() {
			stsBuilder := builder.StatefulSet()
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			Expect(len(statefulSet.OwnerReferences)).To(Equal(1))
			Expect(statefulSet.OwnerReferences[0].Name).To(Equal(builder.Instance.Name))
		})

		It("specifies the upgrade policy", func() {
			stsBuilder := builder.StatefulSet()
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())
			updateStrategy := appsv1.StatefulSetUpdateStrategy{
				RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
					Partition: pointer.Int32Ptr(0),
				},
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			}

			Expect(statefulSet.Spec.UpdateStrategy).To(Equal(updateStrategy))
		})

		It("updates tolerations", func() {
			newToleration := corev1.Toleration{
				Key:      "update",
				Operator: "equals",
				Value:    "works",
				Effect:   "NoSchedule",
			}
			stsBuilder.Instance.Spec.Tolerations = []corev1.Toleration{newToleration}
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			Expect(statefulSet.Spec.Template.Spec.Tolerations).
				To(ConsistOf(newToleration))
		})

		Context("label inheritance", func() {
			BeforeEach(func() {
				instance = generateRabbitmqCluster()
				instance.Namespace = "foo-namespace"
				instance.Name = "foo"
				instance.Labels = map[string]string{
					"app.kubernetes.io/foo": "bar",
					"foo":                   "bar",
					"rabbitmq":              "is-great",
					"foo/app.kubernetes.io": "edgecase",
				}

				scheme = runtime.NewScheme()
				Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
				Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())

				builder = &resource.RabbitmqResourceBuilder{
					Instance: &instance,
					Scheme:   scheme,
				}
			})

			It("restores the default labels", func() {
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())

				labels := statefulSet.Spec.Template.Labels
				Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
				Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
				Expect(labels["app.kubernetes.io/part-of"]).To(Equal("rabbitmq"))
			})

			It("deletes the labels that are removed from the CR", func() {
				existingLabels := map[string]string{
					"app.kubernetes.io/name":      instance.Name,
					"app.kubernetes.io/part-of":   "rabbitmq",
					"this-was-the-previous-label": "should-be-deleted",
				}

				statefulSet.Labels = existingLabels
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())

				Expect(statefulSet.Spec.Template.Labels).NotTo(HaveKey("this-was-the-previous-label"))
			})

			It("has the labels from the instance on the statefulset", func() {
				stsBuilder := builder.StatefulSet()
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())
				testLabels(statefulSet.Labels)
			})

			It("adds default labels to pods but does not populate labels from the instance onto pods", func() {
				stsBuilder := builder.StatefulSet()
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())

				Expect(statefulSet.Spec.Template.ObjectMeta.Labels).To(SatisfyAll(
					HaveLen(3),
					HaveKeyWithValue("app.kubernetes.io/name", instance.Name),
					HaveKeyWithValue("app.kubernetes.io/component", "rabbitmq"),
					HaveKeyWithValue("app.kubernetes.io/part-of", "rabbitmq"),
					Not(HaveKey("foo")),
					Not(HaveKey("rabbitmq")),
					Not(HaveKey("foo/app.kubernetes.io")),
					Not(HaveKey("app.kubernetes.io/foo")),
				))
			})

			It("adds the correct labels on the statefulset", func() {
				stsBuilder.Instance.Labels = map[string]string{
					"app.kubernetes.io/foo": "bar",
					"foo":                   "bar",
					"rabbitmq":              "is-great",
					"foo/app.kubernetes.io": "edgecase",
				}

				existingLabels := map[string]string{
					"app.kubernetes.io/name":      instance.Name,
					"app.kubernetes.io/part-of":   "rabbitmq",
					"this-was-the-previous-label": "should-be-deleted",
				}
				statefulSet.Labels = existingLabels

				Expect(stsBuilder.Update(statefulSet)).To(Succeed())

				By("updating labels from the CR to the statefulset")
				testLabels(statefulSet.Labels)

				By("restoring the default labels")
				labels := statefulSet.Labels
				Expect(labels["app.kubernetes.io/name"]).To(Equal("foo"))
				Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
				Expect(labels["app.kubernetes.io/part-of"]).To(Equal("rabbitmq"))

				By("deleting the labels that are removed from the CR")
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())
				Expect(statefulSet.Labels).NotTo(HaveKey("this-was-the-previous-label"))
			})
		})

		Context("annotations", func() {
			Context("default annotations", func() {

				BeforeEach(func() {
					statefulSet.Spec.Template.Annotations = nil
				})

				It("Adds the default annotations to the pod template", func() {
					Expect(stsBuilder.Update(statefulSet)).To(Succeed())

					Expect(statefulSet.Spec.Template.Annotations).To(HaveKeyWithValue("prometheus.io/scrape", "true"))
					Expect(statefulSet.Spec.Template.Annotations).To(HaveKeyWithValue("prometheus.io/port", "15692"))
				})
			})

			Context("annotation inheritance", func() {
				var (
					existingAnnotations            map[string]string
					existingPodTemplateAnnotations map[string]string
					existingPvcTemplateAnnotations map[string]string
				)

				BeforeEach(func() {
					existingAnnotations = map[string]string{
						"this-was-the-previous-annotation": "should-be-preserved",
						"app.kubernetes.io/part-of":        "rabbitmq",
						"app.k8s.io/something":             "something-amazing",
					}

					existingPodTemplateAnnotations = map[string]string{
						"prometheus.io/scrape":           "true",
						"prometheus.io/port":             "15692",
						"this-was-the-previous-pod-anno": "should-be-preserved",
					}

					existingPvcTemplateAnnotations = map[string]string{
						"this-was-the-previous-pod-anno": "should-be-preserved-here",
						"app.kubernetes.io/part-of":      "rabbitmq-pvc",
						"app.k8s.io/something":           "something-amazing-on-pvc",
					}

					statefulSet.Annotations = existingAnnotations
					statefulSet.Spec.Template.Annotations = existingPodTemplateAnnotations
					statefulSet.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
						{},
					}
					statefulSet.Spec.VolumeClaimTemplates[0].Annotations = existingPvcTemplateAnnotations
				})

				It("updates sts annotations", func() {
					stsBuilder.Instance.Annotations = map[string]string{
						"kubernetes.io/name":          "i-do-not-like-this",
						"kubectl.kubernetes.io/name":  "i-do-not-like-this",
						"k8s.io/name":                 "i-do-not-like-this",
						"kubernetes.io/other":         "i-do-not-like-this",
						"kubectl.kubernetes.io/other": "i-do-not-like-this",
						"k8s.io/other":                "i-do-not-like-this",
						"my-annotation":               "i-like-this",
					}
					Expect(stsBuilder.Update(statefulSet)).To(Succeed())

					expectedAnnotations := map[string]string{
						"my-annotation":                    "i-like-this",
						"this-was-the-previous-annotation": "should-be-preserved",
						"app.kubernetes.io/part-of":        "rabbitmq",
						"app.k8s.io/something":             "something-amazing",
					}

					Expect(statefulSet.Annotations).To(Equal(expectedAnnotations))
				})

				It("adds default annotations but does not populate annotations from the instance to the pod", func() {
					stsBuilder.Instance.Annotations = map[string]string{
						"my-annotation": "my-annotation",
						"k8s.io/other":  "i-do-not-like-this",
					}

					Expect(stsBuilder.Update(statefulSet)).To(Succeed())
					Expect(statefulSet.Spec.Template.Annotations).To(SatisfyAll(
						HaveLen(3),
						HaveKeyWithValue("prometheus.io/scrape", "true"),
						HaveKeyWithValue("prometheus.io/port", "15692"),
						HaveKeyWithValue("this-was-the-previous-pod-anno", "should-be-preserved"),
						Not(HaveKey("app.kubernetes.io/part-of")),
						Not(HaveKey("app.k8s.io/something")),
						Not(HaveKey("my-annotation")),
						Not(HaveKey("k8s.io/other")),
					))
				})

				It("does not update annotations from the instance to the pvc template", func() {
					stsBuilder.Instance.Annotations = map[string]string{
						"kubernetes.io/name":          "i-do-not-like-this",
						"kubectl.kubernetes.io/name":  "i-do-not-like-this",
						"k8s.io/name":                 "i-do-not-like-this",
						"kubernetes.io/other":         "i-do-not-like-this",
						"kubectl.kubernetes.io/other": "i-do-not-like-this",
						"k8s.io/other":                "i-do-not-like-this",
						"my-annotation":               "i-do-not-like-this",
					}

					Expect(stsBuilder.Update(statefulSet)).To(Succeed())
					expectedAnnotations := map[string]string{
						"app.kubernetes.io/part-of":      "rabbitmq-pvc",
						"this-was-the-previous-pod-anno": "should-be-preserved-here",
						"app.k8s.io/something":           "something-amazing-on-pvc",
					}

					Expect(statefulSet.Spec.VolumeClaimTemplates[0].Annotations).To(Equal(expectedAnnotations))
				})
			})
		})

		Context("TLS", func() {
			It("adds a TLS volume to the pod template spec", func() {
				instance.Spec.TLS.SecretName = "tls-secret"
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())

				filePermissions := int32(400)
				secretEnforced := true
				Expect(statefulSet.Spec.Template.Spec.Volumes).To(ContainElement(corev1.Volume{
					Name: "rabbitmq-tls",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  "tls-secret",
							DefaultMode: &filePermissions,
							Optional:    &secretEnforced,
						},
					},
				}))
			})

			It("adds two TLS volume mounts to the rabbitmq container", func() {
				instance.Spec.TLS.SecretName = "tls-secret"
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())

				rabbitmqContainerSpec := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(rabbitmqContainerSpec.VolumeMounts).To(ContainElement(corev1.VolumeMount{
					Name:      "rabbitmq-tls",
					MountPath: "/etc/rabbitmq-tls/tls.crt",
					SubPath:   "tls.crt",
					ReadOnly:  true,
				}))
				Expect(rabbitmqContainerSpec.VolumeMounts).To(ContainElement(corev1.VolumeMount{
					Name:      "rabbitmq-tls",
					MountPath: "/etc/rabbitmq-tls/tls.key",
					SubPath:   "tls.key",
					ReadOnly:  true,
				}))
			})

			It("opens tls ports for amqps and management-tls on the rabbitmq container", func() {
				instance.Spec.TLS.SecretName = "tls-secret"
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())

				rabbitmqContainerSpec := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(rabbitmqContainerSpec.Ports).To(ContainElements([]corev1.ContainerPort{
					{
						Name:          "amqps",
						ContainerPort: 5671,
					},
					{
						Name:          "management-tls",
						ContainerPort: 15671,
					},
				}))
			})

			It("opens tls ports when mqtt and stomp are configured", func() {
				instance.Spec.TLS.SecretName = "tls-secret"
				instance.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{rabbitmqv1beta1.Plugin("rabbitmq_mqtt"), rabbitmqv1beta1.Plugin("rabbitmq_stomp")}
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())

				rabbitmqContainerSpec := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")

				Expect(rabbitmqContainerSpec.Ports).To(ContainElements([]corev1.ContainerPort{
					{
						Name:          "mqtts",
						ContainerPort: 8883,
					},
					{
						Name:          "stomps",
						ContainerPort: 61614,
					},
				}))
			})

			Context("Mutual TLS (same secret)", func() {
				It("add a TLS CA cert volume mount to the rabbitmq container", func() {
					instance.Spec.TLS.SecretName = "tls-secret"
					instance.Spec.TLS.CaSecretName = "tls-secret"
					Expect(stsBuilder.Update(statefulSet)).To(Succeed())

					rabbitmqContainerSpec := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
					Expect(rabbitmqContainerSpec.VolumeMounts).To(ContainElement(corev1.VolumeMount{
						Name:      "rabbitmq-tls",
						MountPath: "/etc/rabbitmq-tls/ca.crt",
						SubPath:   "ca.crt",
						ReadOnly:  true,
					}))
				})

				It("opens tls ports when rabbitmq_web_mqtt and rabbitmq_web_stomp are configured", func() {
					instance.Spec.TLS.SecretName = "tls-secret"
					instance.Spec.TLS.CaSecretName = "tls-secret"
					instance.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{rabbitmqv1beta1.Plugin("rabbitmq_web_mqtt"), rabbitmqv1beta1.Plugin("rabbitmq_web_stomp")}
					Expect(stsBuilder.Update(statefulSet)).To(Succeed())

					rabbitmqContainerSpec := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")

					Expect(rabbitmqContainerSpec.Ports).To(ContainElements([]corev1.ContainerPort{
						{
							Name:          "web-mqtt-tls",
							ContainerPort: 15676,
						},
						{
							Name:          "web-stomp-tls",
							ContainerPort: 15673,
						},
					}))
				})
			})

			Context("Mutual TLS (different secret)", func() {
				It("add a TLS CA cert volume mount to the rabbitmq container", func() {
					instance.Spec.TLS.SecretName = "tls-secret"
					instance.Spec.TLS.CaSecretName = "mutual-tls-secret"
					Expect(stsBuilder.Update(statefulSet)).To(Succeed())

					rabbitmqContainerSpec := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
					Expect(rabbitmqContainerSpec.VolumeMounts).To(ContainElement(corev1.VolumeMount{
						Name:      "rabbitmq-mutual-tls",
						MountPath: "/etc/rabbitmq-tls/ca.crt",
						SubPath:   "ca.crt",
						ReadOnly:  true,
					}))
				})

				It("adds a mutual TLS volume to the pod template spec", func() {
					instance.Spec.TLS.SecretName = "tls-secret"
					instance.Spec.TLS.CaSecretName = "mutual-tls-secret"
					Expect(stsBuilder.Update(statefulSet)).To(Succeed())

					filePermissions := int32(400)
					secretEnforced := true
					Expect(statefulSet.Spec.Template.Spec.Volumes).To(ContainElement(corev1.Volume{
						Name: "rabbitmq-mutual-tls",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName:  "mutual-tls-secret",
								DefaultMode: &filePermissions,
								Optional:    &secretEnforced,
							},
						},
					}))
				})
			})

			When("DisableNonTLSListeners is set to true", func() {
				BeforeEach(func() {
					instance.Spec.TLS.SecretName = "tls-secret"
					instance.Spec.TLS.DisableNonTLSListeners = true
				})
				It("disables non tls ports for amqp and management in statefulSet", func() {
					Expect(stsBuilder.Update(statefulSet)).To(Succeed())
					rabbitmqContainerSpec := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
					Expect(rabbitmqContainerSpec.Ports).To(ConsistOf([]corev1.ContainerPort{
						{
							Name:          "epmd",
							ContainerPort: 4369,
						},
						{
							Name:          "prometheus",
							ContainerPort: 15692,
						},
						{
							Name:          "amqps",
							ContainerPort: 5671,
						},
						{
							Name:          "management-tls",
							ContainerPort: 15671,
						},
					}))
				})

				It("disables non tls ports for mqtt and stomp when configured", func() {
					instance.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{rabbitmqv1beta1.Plugin("rabbitmq_mqtt"), rabbitmqv1beta1.Plugin("rabbitmq_stomp")}
					Expect(stsBuilder.Update(statefulSet)).To(Succeed())

					rabbitmqContainerSpec := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
					Expect(rabbitmqContainerSpec.Ports).To(ConsistOf([]corev1.ContainerPort{
						{
							Name:          "epmd",
							ContainerPort: 4369,
						},
						{
							Name:          "prometheus",
							ContainerPort: 15692,
						},
						{
							Name:          "amqps",
							ContainerPort: 5671,
						},
						{
							Name:          "management-tls",
							ContainerPort: 15671,
						},
						{
							Name:          "mqtts",
							ContainerPort: 8883,
						},
						{
							Name:          "stomps",
							ContainerPort: 61614,
						},
					}))
				})

				It("sets tcp readiness probe to use port amqps", func() {
					Expect(stsBuilder.Update(statefulSet)).To(Succeed())
					container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
					TCPProbe := container.ReadinessProbe.TCPSocket
					Expect(TCPProbe.Port.Type).To(Equal(intstr.String))
					Expect(TCPProbe.Port.StrVal).To(Equal("amqps"))
				})
			})
		})

		It("updates the imagePullSecrets list; sets it back to empty list after deleting the configuration", func() {
			stsBuilder.Instance.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "my-shiny-new-secret"}}
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())
			Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "my-shiny-new-secret"}))

			stsBuilder.Instance.Spec.ImagePullSecrets = nil
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())
			Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(BeEmpty())
		})

		It("sets replicas", func() {
			stsBuilder := builder.StatefulSet()
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			Expect(*statefulSet.Spec.Replicas).To(Equal(int32(1)))
		})

		It("sets a TopologySpreadConstraint", func() {
			stsBuilder := builder.StatefulSet()
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			Expect(statefulSet.Spec.Template.Spec.TopologySpreadConstraints).To(ConsistOf(
				corev1.TopologySpreadConstraint{
					MaxSkew:           1,
					TopologyKey:       "topology.kubernetes.io/zone",
					WhenUnsatisfiable: corev1.ScheduleAnyway,
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": instance.Name,
						},
					},
				}))
		})

		It("has resources requirements on the init container", func() {
			stsBuilder := builder.StatefulSet()
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			resources := statefulSet.Spec.Template.Spec.InitContainers[0].Resources
			Expect(resources.Requests["cpu"]).To(Equal(k8sresource.MustParse("100m")))
			Expect(resources.Requests["memory"]).To(Equal(k8sresource.MustParse("500Mi")))
			Expect(resources.Limits["cpu"]).To(Equal(k8sresource.MustParse("100m")))
			Expect(resources.Limits["memory"]).To(Equal(k8sresource.MustParse("500Mi")))
		})

		It("exposes required Container Ports", func() {
			stsBuilder := builder.StatefulSet()
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			requiredContainerPorts := []int32{4369, 5672, 15672, 15692}
			var actualContainerPorts []int32

			container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
			for _, port := range container.Ports {
				actualContainerPorts = append(actualContainerPorts, port.ContainerPort)
			}

			Expect(actualContainerPorts).To(ConsistOf(requiredContainerPorts))
		})

		DescribeTable("plugins exposing ports",
			func(plugin, containerPortName string, port int) {
				instance.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{rabbitmqv1beta1.Plugin(plugin)}
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())

				expectedPort := corev1.ContainerPort{
					Name:          containerPortName,
					ContainerPort: int32(port),
				}
				container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(container.Ports).To(ContainElement(expectedPort))
			},
			Entry("MQTT", "rabbitmq_mqtt", "mqtt", 1883),
			Entry("MQTT-over-WebSockets", "rabbitmq_web_mqtt", "web-mqtt", 15675),
			Entry("STOMP", "rabbitmq_stomp", "stomp", 61613),
			Entry("STOMP-over-WebSockets", "rabbitmq_web_stomp", "web-stomp", 15674),
		)

		It("uses required Environment Variables", func() {
			stsBuilder := builder.StatefulSet()
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			requiredEnvVariables := []corev1.EnvVar{
				{
					Name: "MY_POD_NAME",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath:  "metadata.name",
							APIVersion: "v1",
						},
					},
				},
				{
					Name: "MY_POD_NAMESPACE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath:  "metadata.namespace",
							APIVersion: "v1",
						},
					},
				},
				{
					Name:  "RABBITMQ_ENABLED_PLUGINS_FILE",
					Value: "/operator/enabled_plugins",
				},
				{
					Name:  "K8S_SERVICE_NAME",
					Value: instance.ChildResourceName("nodes"),
				},
				{
					Name:  "RABBITMQ_USE_LONGNAME",
					Value: "true",
				},
				{
					Name:  "RABBITMQ_NODENAME",
					Value: "rabbit@$(MY_POD_NAME).$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE)",
				},
				{
					Name:  "K8S_HOSTNAME_SUFFIX",
					Value: ".$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE)",
				},
			}

			container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
			Expect(container.Env).To(ConsistOf(requiredEnvVariables))
		})

		Context("Rabbitmq container volume mounts", func() {
			DescribeTable("Volume mounts depending on spec configuration and '/var/lib/rabbitmq/' always mounts before '/var/lib/rabbitmq/mnesia/' ",
				func(rabbitmqEnv, advancedConfig string) {
					stsBuilder := builder.StatefulSet()
					stsBuilder.Instance.Spec.Rabbitmq.EnvConfig = rabbitmqEnv
					stsBuilder.Instance.Spec.Rabbitmq.AdvancedConfig = advancedConfig
					Expect(stsBuilder.Update(statefulSet)).To(Succeed())

					expectedVolumeMounts := []corev1.VolumeMount{
						{Name: "persistence", MountPath: "/var/lib/rabbitmq/mnesia/"},
						{Name: "rabbitmq-erlang-cookie", MountPath: "/var/lib/rabbitmq/"},
						{Name: "pod-info", MountPath: "/etc/pod-info/"},
						{Name: "rabbitmq-confd", MountPath: "/etc/rabbitmq/conf.d/default_user.conf", SubPath: "default_user.conf"},
						{Name: "server-conf", MountPath: "/etc/rabbitmq/rabbitmq.conf", SubPath: "rabbitmq.conf"},
						{Name: "rabbitmq-plugins", MountPath: "/operator"},
					}

					if rabbitmqEnv != "" {
						expectedVolumeMounts = append(expectedVolumeMounts, corev1.VolumeMount{
							Name: "server-conf", MountPath: "/etc/rabbitmq/rabbitmq-env.conf", SubPath: "rabbitmq-env.conf"})
					}

					if advancedConfig != "" {
						expectedVolumeMounts = append(expectedVolumeMounts, corev1.VolumeMount{
							Name: "server-conf", MountPath: "/etc/rabbitmq/advanced.config", SubPath: "advanced.config"})
					}

					container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
					Expect(container.VolumeMounts).To(ConsistOf(expectedVolumeMounts))
					Expect(container.VolumeMounts[0]).To(Equal(corev1.VolumeMount{
						Name:      "rabbitmq-erlang-cookie",
						MountPath: "/var/lib/rabbitmq/",
					}))
					Expect(container.VolumeMounts[1]).To(Equal(corev1.VolumeMount{
						Name:      "persistence",
						MountPath: "/var/lib/rabbitmq/mnesia/",
					}))
				},
				Entry("Both env and advanced configs are set", "rabbitmq-env-is-set", "advanced-config-is-set"),
				Entry("Only env config is set", "rabbitmq-env-is-set", ""),
				Entry("Only advanced config is set", "", "advanced-config-is-set"),
				Entry("No configs are set", "", ""),
			)
		})

		It("defines the expected volumes", func() {
			stsBuilder := builder.StatefulSet()
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			Expect(statefulSet.Spec.Template.Spec.Volumes).To(ConsistOf(
				corev1.Volume{
					Name: "server-conf",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: instance.ChildResourceName("server-conf"),
							},
						},
					},
				},
				corev1.Volume{
					Name: "plugins-conf",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: instance.ChildResourceName("plugins-conf"),
							},
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
											Name: builder.Instance.ChildResourceName("default-user"),
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
						},
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
							SecretName: instance.ChildResourceName("erlang-cookie"),
						},
					},
				},
				corev1.Volume{
					Name: "rabbitmq-plugins",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				corev1.Volume{
					Name: "pod-info",
					VolumeSource: corev1.VolumeSource{
						DownwardAPI: &corev1.DownwardAPIVolumeSource{
							Items: []corev1.DownwardAPIVolumeFile{
								{
									Path: "skipPreStopChecks",
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.labels['skipPreStopChecks']",
									},
								},
							},
						},
					},
				},
			))
		})

		It("uses the correct service account", func() {
			stsBuilder := builder.StatefulSet()
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			Expect(statefulSet.Spec.Template.Spec.ServiceAccountName).To(Equal(instance.ChildResourceName("server")))
		})

		It("mounts the service account in its pods", func() {
			stsBuilder := builder.StatefulSet()
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			Expect(*statefulSet.Spec.Template.Spec.AutomountServiceAccountToken).To(BeTrue())
		})

		It("creates the required SecurityContext", func() {
			stsBuilder := builder.StatefulSet()
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			rmqGID, rmqUID := int64(999), int64(999)

			expectedPodSecurityContext := &corev1.PodSecurityContext{
				FSGroup:    &rmqGID,
				RunAsGroup: &rmqGID,
				RunAsUser:  &rmqUID,
			}

			Expect(statefulSet.Spec.Template.Spec.SecurityContext).To(Equal(expectedPodSecurityContext))
		})

		It("defines a Readiness Probe", func() {
			stsBuilder := builder.StatefulSet()
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
			TCPProbe := container.ReadinessProbe.TCPSocket
			Expect(TCPProbe.Port.Type).To(Equal(intstr.String))
			Expect(TCPProbe.Port.StrVal).To(Equal("amqp"))
		})

		It("templates the correct InitContainer", func() {
			stsBuilder := builder.StatefulSet()
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			initContainers := statefulSet.Spec.Template.Spec.InitContainers
			Expect(initContainers).To(HaveLen(1))

			initContainer := extractContainer(initContainers, "setup-container")
			Expect(initContainer).To(MatchFields(IgnoreExtras, Fields{
				"Image": Equal("rabbitmq-image-from-cr"),
				"SecurityContext": PointTo(MatchFields(IgnoreExtras, Fields{
					"Capabilities": PointTo(MatchAllFields(Fields{
						"Drop": ConsistOf([]corev1.Capability{
							"FSETID",
							"KILL",
							"SETGID",
							"SETUID",
							"SETPCAP",
							"NET_BIND_SERVICE",
							"NET_RAW",
							"SYS_CHROOT",
							"MKNOD",
							"AUDIT_WRITE",
							"SETFCAP",
						}),
						"Add": BeEmpty(),
					})),
				})),
				"Command": ConsistOf(
					"sh", "-c", "cp /tmp/erlang-cookie-secret/.erlang.cookie /var/lib/rabbitmq/.erlang.cookie "+
						"&& chown 999:999 /var/lib/rabbitmq/.erlang.cookie "+
						"&& chmod 600 /var/lib/rabbitmq/.erlang.cookie ; "+
						"cp /tmp/rabbitmq-plugins/enabled_plugins /operator/enabled_plugins "+
						"&& chown 999:999 /operator/enabled_plugins ; "+
						"chgrp 999 /var/lib/rabbitmq/mnesia/ ; "+
						"echo '[default]' > /var/lib/rabbitmq/.rabbitmqadmin.conf "+
						"&& sed -e 's/default_user/username/' -e 's/default_pass/password/' /tmp/default_user.conf >> /var/lib/rabbitmq/.rabbitmqadmin.conf "+
						"&& chown 999:999 /var/lib/rabbitmq/.rabbitmqadmin.conf "+
						"&& chmod 600 /var/lib/rabbitmq/.rabbitmqadmin.conf",
				),
				"VolumeMounts": ConsistOf(
					corev1.VolumeMount{
						Name:      "plugins-conf",
						MountPath: "/tmp/rabbitmq-plugins/",
					},
					corev1.VolumeMount{
						Name:      "rabbitmq-erlang-cookie",
						MountPath: "/var/lib/rabbitmq/",
					},
					corev1.VolumeMount{
						Name:      "erlang-cookie-secret",
						MountPath: "/tmp/erlang-cookie-secret/",
					},
					corev1.VolumeMount{
						Name:      "rabbitmq-plugins",
						MountPath: "/operator",
					},
					corev1.VolumeMount{
						Name:      "persistence",
						MountPath: "/var/lib/rabbitmq/mnesia/",
					},
					corev1.VolumeMount{
						Name:      "rabbitmq-confd",
						MountPath: "/tmp/default_user.conf",
						SubPath:   "default_user.conf",
					},
				),
			}))
		})

		It("sets TerminationGracePeriodSeconds in podTemplate as provided in instance spec", func() {
			instance.Spec.TerminationGracePeriodSeconds = pointer.Int64Ptr(10)
			builder = &resource.RabbitmqResourceBuilder{
				Instance: &instance,
				Scheme:   scheme,
			}

			stsBuilder := builder.StatefulSet()
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			gracePeriodSeconds := statefulSet.Spec.Template.Spec.TerminationGracePeriodSeconds
			Expect(gracePeriodSeconds).To(Equal(pointer.Int64Ptr(10)))

			// TerminationGracePeriodSeconds is used to set commands timeouts in the preStop hook
			expectedPreStopCommand := []string{"/bin/bash", "-c", "if [ ! -z \"$(cat /etc/pod-info/skipPreStopChecks)\" ]; then exit 0; fi; rabbitmq-upgrade await_online_quorum_plus_one -t 10; rabbitmq-upgrade await_online_synchronized_mirror -t 10; rabbitmq-upgrade drain -t 10"}
			Expect(statefulSet.Spec.Template.Spec.Containers[0].Lifecycle.PreStop.Exec.Command).To(Equal(expectedPreStopCommand))
		})

		It("checks mirror and querum queue status in preStop hook", func() {
			stsBuilder := builder.StatefulSet()
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			expectedPreStopCommand := []string{"/bin/bash", "-c", "if [ ! -z \"$(cat /etc/pod-info/skipPreStopChecks)\" ]; then exit 0; fi; rabbitmq-upgrade await_online_quorum_plus_one -t 604800; rabbitmq-upgrade await_online_synchronized_mirror -t 604800; rabbitmq-upgrade drain -t 604800"}

			Expect(statefulSet.Spec.Template.Spec.Containers[0].Lifecycle.PreStop.Exec.Command).To(Equal(expectedPreStopCommand))
		})

		Context("resources requirements", func() {
			It("sets StatefulSet resource requirements", func() {
				instance.Spec.Resources = &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    k8sresource.MustParse("10m"),
						corev1.ResourceMemory: k8sresource.MustParse("3Gi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    k8sresource.MustParse("11m"),
						corev1.ResourceMemory: k8sresource.MustParse("4Gi"),
					},
				}

				builder = &resource.RabbitmqResourceBuilder{
					Instance: &instance,
					Scheme:   scheme,
				}

				stsBuilder := builder.StatefulSet()
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())
				expectedCPURequest, _ := k8sresource.ParseQuantity("10m")
				expectedMemoryRequest, _ := k8sresource.ParseQuantity("3Gi")
				expectedCPULimit, _ := k8sresource.ParseQuantity("11m")
				expectedMemoryLimit, _ := k8sresource.ParseQuantity("4Gi")

				container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(container.Resources.Requests[corev1.ResourceCPU]).To(Equal(expectedCPURequest))
				Expect(container.Resources.Requests[corev1.ResourceMemory]).To(Equal(expectedMemoryRequest))
				Expect(container.Resources.Limits[corev1.ResourceCPU]).To(Equal(expectedCPULimit))
				Expect(container.Resources.Limits[corev1.ResourceMemory]).To(Equal(expectedMemoryLimit))
			})

			It("does not set any resource requirements if empty maps are provided in the CR", func() {
				instance.Spec.Resources = &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{},
					Limits:   corev1.ResourceList{},
				}

				builder = &resource.RabbitmqResourceBuilder{
					Instance: &instance,
					Scheme:   scheme,
				}

				stsBuilder := builder.StatefulSet()
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())

				container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(len(container.Resources.Requests)).To(Equal(0))
				Expect(len(container.Resources.Limits)).To(Equal(0))
			})
		})

		When("configures private image", func() {
			It("uses the instance ImagePullSecrets and image reference when provided", func() {
				instance.Spec.Image = "my-private-repo/rabbitmq:latest"
				instance.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "my-great-secret"}}
				builder = &resource.RabbitmqResourceBuilder{
					Instance: &instance,
					Scheme:   scheme,
				}

				stsBuilder := builder.StatefulSet()
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())
				container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(container.Image).To(Equal("my-private-repo/rabbitmq:latest"))
				Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "my-great-secret"}))
			})
		})

		It("sets the replica count of the StatefulSet to the instance value", func() {
			instance.Spec.Replicas = pointer.Int32Ptr(3)
			builder = &resource.RabbitmqResourceBuilder{
				Instance: &instance,
				Scheme:   scheme,
			}
			stsBuilder := builder.StatefulSet()
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())
			Expect(*statefulSet.Spec.Replicas).To(Equal(int32(3)))
		})

		When("stateful set override are provided", func() {
			It("overrides statefulSet.ObjectMeta.Annotations", func() {
				instance.Annotations = map[string]string{
					"key1":    "value1",
					"keep-me": "keep-me",
				}
				instance.Spec.Override.StatefulSet = &rabbitmqv1beta1.StatefulSet{
					EmbeddedLabelsAnnotations: &rabbitmqv1beta1.EmbeddedLabelsAnnotations{
						Annotations: map[string]string{
							"new-key": "new-value",
							"key1":    "new-value",
						},
					},
				}
				stsBuilder := builder.StatefulSet()
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())
				Expect(statefulSet.ObjectMeta.Name).To(Equal(instance.Name))
				Expect(statefulSet.ObjectMeta.Namespace).To(Equal(instance.Namespace))
				Expect(statefulSet.ObjectMeta.Labels).To(Equal(map[string]string{
					"app.kubernetes.io/name":      instance.Name,
					"app.kubernetes.io/component": "rabbitmq",
					"app.kubernetes.io/part-of":   "rabbitmq",
				}))

				Expect(statefulSet.ObjectMeta.Annotations).To(Equal(map[string]string{
					"new-key": "new-value",
					"key1":    "new-value",
					"keep-me": "keep-me",
				}))
			})

			It("overrides statefulSet.ObjectMeta.Labels", func() {
				instance.Annotations = map[string]string{"my-key": "my-value"}
				instance.Labels = map[string]string{
					"key1":    "value1",
					"keep-me": "keep-me",
				}
				instance.Spec.Override.StatefulSet = &rabbitmqv1beta1.StatefulSet{
					EmbeddedLabelsAnnotations: &rabbitmqv1beta1.EmbeddedLabelsAnnotations{
						Labels: map[string]string{
							"new-label-key": "new-label-value",
							"key1":          "new-value",
						},
					},
				}
				stsBuilder := builder.StatefulSet()
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())
				Expect(statefulSet.ObjectMeta.Name).To(Equal(instance.Name))
				Expect(statefulSet.ObjectMeta.Namespace).To(Equal(instance.Namespace))
				Expect(statefulSet.ObjectMeta.Annotations).To(Equal(map[string]string{"my-key": "my-value"}))
				Expect(statefulSet.ObjectMeta.Labels).To(Equal(map[string]string{
					"new-label-key":               "new-label-value",
					"key1":                        "new-value",
					"keep-me":                     "keep-me",
					"app.kubernetes.io/component": "rabbitmq",
					"app.kubernetes.io/part-of":   "rabbitmq",
					"app.kubernetes.io/name":      "foo",
				}))
			})

			It("overrides statefulSet.spec.replicas", func() {
				instance.Spec.Override.StatefulSet = &rabbitmqv1beta1.StatefulSet{
					Spec: &rabbitmqv1beta1.StatefulSetSpec{
						Replicas: pointer.Int32Ptr(10),
					},
				}

				stsBuilder := builder.StatefulSet()
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())
				Expect(*statefulSet.Spec.Replicas).To(Equal(int32(10)))
			})

			It("overrides statefulSet.spec.podManagementPolicy", func() {
				instance.Spec.Override.StatefulSet = &rabbitmqv1beta1.StatefulSet{
					Spec: &rabbitmqv1beta1.StatefulSetSpec{
						PodManagementPolicy: "my-policy",
					},
				}

				stsBuilder := builder.StatefulSet()
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())
				Expect(string(statefulSet.Spec.PodManagementPolicy)).To(Equal("my-policy"))
			})

			It("overrides statefulSet.spec.UpdateStrategy", func() {
				instance.Spec.Override.StatefulSet = &rabbitmqv1beta1.StatefulSet{
					Spec: &rabbitmqv1beta1.StatefulSetSpec{
						UpdateStrategy: &appsv1.StatefulSetUpdateStrategy{
							Type: "OnDelete",
							RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
								Partition: pointer.Int32Ptr(1),
							},
						},
					},
				}

				stsBuilder := builder.StatefulSet()
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())
				Expect(string(statefulSet.Spec.UpdateStrategy.Type)).To(Equal("OnDelete"))
				Expect(*statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition).To(Equal(int32(1)))
			})

			Context("PodTemplateSpec", func() {
				It("Overrides PodTemplateSpec objectMeta", func() {
					instance.Spec.Override.StatefulSet = &rabbitmqv1beta1.StatefulSet{
						Spec: &rabbitmqv1beta1.StatefulSetSpec{
							Template: &rabbitmqv1beta1.PodTemplateSpec{
								EmbeddedObjectMeta: &rabbitmqv1beta1.EmbeddedObjectMeta{
									Namespace: "my-ns",
									Name:      "my-name",
									Labels: map[string]string{
										"my-label": "my-label",
									},
									Annotations: map[string]string{
										"my-key": "my-value",
									},
								},
							},
						},
					}
					stsBuilder := builder.StatefulSet()
					Expect(stsBuilder.Update(statefulSet)).To(Succeed())
					Expect(statefulSet.Spec.Template.ObjectMeta.Name).To(Equal("my-name"))
					Expect(statefulSet.Spec.Template.ObjectMeta.Namespace).To(Equal("my-ns"))
					Expect(statefulSet.Spec.Template.ObjectMeta.Labels).To(Equal(map[string]string{
						"my-label":                    "my-label",
						"app.kubernetes.io/component": "rabbitmq",
						"app.kubernetes.io/part-of":   "rabbitmq",
						"app.kubernetes.io/name":      "foo",
					}))
					Expect(statefulSet.Spec.Template.ObjectMeta.Annotations).To(Equal(map[string]string{
						"my-key":               "my-value",
						"prometheus.io/scrape": "true",
						"prometheus.io/port":   "15692",
					}))
				})
				It("Overrides PodSpec", func() {
					instance.Spec.Override.StatefulSet = &rabbitmqv1beta1.StatefulSet{
						Spec: &rabbitmqv1beta1.StatefulSetSpec{
							Template: &rabbitmqv1beta1.PodTemplateSpec{
								Spec: &corev1.PodSpec{
									TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
										{
											MaxSkew:           1,
											TopologyKey:       "my-topology",
											WhenUnsatisfiable: corev1.DoNotSchedule,
											LabelSelector: &metav1.LabelSelector{
												MatchLabels: map[string]string{
													"key": "value",
												},
											},
										},
									},
									Containers: []corev1.Container{
										{
											Name: "rabbitmq",
											Env: []corev1.EnvVar{
												{
													Name:  "test1",
													Value: "test1",
												},
											},
											VolumeMounts: []corev1.VolumeMount{
												{
													Name:      "test",
													MountPath: "test-path",
												},
											},
										},
										{
											Name:  "new-container-0",
											Image: "my-image-0",
										},
										{
											Name:  "new-container-1",
											Image: "my-image-1",
										},
									},
								},
							},
						},
					}

					builder = &resource.RabbitmqResourceBuilder{
						Instance: &instance,
						Scheme:   scheme,
					}
					stsBuilder := builder.StatefulSet()
					Expect(stsBuilder.Update(statefulSet)).To(Succeed())

					Expect(statefulSet.Spec.Template.Spec.TopologySpreadConstraints).To(ConsistOf(
						corev1.TopologySpreadConstraint{
							MaxSkew:           1,
							TopologyKey:       "my-topology",
							WhenUnsatisfiable: corev1.DoNotSchedule,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"key": "value",
								},
							},
						},
						corev1.TopologySpreadConstraint{
							MaxSkew:           1,
							TopologyKey:       "topology.kubernetes.io/zone",
							WhenUnsatisfiable: corev1.ScheduleAnyway,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": instance.Name,
								},
							},
						},
					))

					Expect(extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq").Env).To(ConsistOf(
						corev1.EnvVar{
							Name:  "test1",
							Value: "test1",
						},
						corev1.EnvVar{
							Name: "MY_POD_NAME",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath:  "metadata.name",
									APIVersion: "v1",
								},
							},
						},
						corev1.EnvVar{
							Name: "MY_POD_NAMESPACE",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath:  "metadata.namespace",
									APIVersion: "v1",
								},
							},
						},
						corev1.EnvVar{
							Name:  "K8S_SERVICE_NAME",
							Value: instance.ChildResourceName("nodes"),
						},
						corev1.EnvVar{
							Name:  "RABBITMQ_USE_LONGNAME",
							Value: "true",
						},
						corev1.EnvVar{
							Name:  "RABBITMQ_ENABLED_PLUGINS_FILE",
							Value: "/operator/enabled_plugins",
						},
						corev1.EnvVar{
							Name:  "RABBITMQ_NODENAME",
							Value: "rabbit@$(MY_POD_NAME).$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE)",
						},
						corev1.EnvVar{
							Name:  "K8S_HOSTNAME_SUFFIX",
							Value: ".$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE)",
						}))
					Expect(extractContainer(statefulSet.Spec.Template.Spec.Containers, "new-container-0")).To(Equal(
						corev1.Container{Name: "new-container-0", Image: "my-image-0"}))
					Expect(extractContainer(statefulSet.Spec.Template.Spec.Containers, "new-container-1")).To(Equal(
						corev1.Container{Name: "new-container-1", Image: "my-image-1"}))
					Expect(extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq").VolumeMounts).To(ConsistOf(
						corev1.VolumeMount{
							Name:      "test",
							MountPath: "test-path",
						},
						corev1.VolumeMount{
							Name:      "persistence",
							MountPath: "/var/lib/rabbitmq/mnesia/",
						},
						corev1.VolumeMount{
							Name:      "rabbitmq-confd",
							MountPath: "/etc/rabbitmq/conf.d/default_user.conf",
							SubPath:   "default_user.conf",
						},
						corev1.VolumeMount{
							Name:      "rabbitmq-erlang-cookie",
							MountPath: "/var/lib/rabbitmq/",
						},
						corev1.VolumeMount{
							Name:      "pod-info",
							MountPath: "/etc/pod-info/",
						},
						corev1.VolumeMount{
							Name:      "server-conf",
							MountPath: "/etc/rabbitmq/rabbitmq.conf",
							SubPath:   "rabbitmq.conf",
						},
						corev1.VolumeMount{
							Name:      "rabbitmq-plugins",
							MountPath: "/operator",
						},
					))
				})
				Context("Rabbitmq Container volume mounts", func() {
					It("Overrides the volume mounts list while making sure that '/var/lib/rabbitmq/' mounts before '/var/lib/rabbitmq/mnesia/' ", func() {
						instance.Spec.Override.StatefulSet = &rabbitmqv1beta1.StatefulSet{
							Spec: &rabbitmqv1beta1.StatefulSetSpec{
								Template: &rabbitmqv1beta1.PodTemplateSpec{
									Spec: &corev1.PodSpec{
										Containers: []corev1.Container{
											{
												Name: "rabbitmq",
												VolumeMounts: []corev1.VolumeMount{
													{
														Name:      "test",
														MountPath: "test",
													},
												},
											},
										},
									},
								},
							},
						}

						builder = &resource.RabbitmqResourceBuilder{
							Instance: &instance,
							Scheme:   scheme,
						}
						stsBuilder := builder.StatefulSet()
						Expect(stsBuilder.Update(statefulSet)).To(Succeed())
						expectedVolumeMounts := []corev1.VolumeMount{
							{Name: "persistence", MountPath: "/var/lib/rabbitmq/mnesia/"},
							{Name: "rabbitmq-erlang-cookie", MountPath: "/var/lib/rabbitmq/"},
							{Name: "pod-info", MountPath: "/etc/pod-info/"},
							{Name: "rabbitmq-confd", MountPath: "/etc/rabbitmq/conf.d/default_user.conf", SubPath: "default_user.conf"},
							{Name: "server-conf", MountPath: "/etc/rabbitmq/rabbitmq.conf", SubPath: "rabbitmq.conf"},
							{Name: "rabbitmq-plugins", MountPath: "/operator"},
							{Name: "test", MountPath: "test"},
						}

						container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
						Expect(container.VolumeMounts).To(ConsistOf(expectedVolumeMounts))
						Expect(container.VolumeMounts[0]).To(Equal(corev1.VolumeMount{
							Name:      "rabbitmq-erlang-cookie",
							MountPath: "/var/lib/rabbitmq/",
						}))
						Expect(container.VolumeMounts[1]).To(Equal(corev1.VolumeMount{
							Name:      "persistence",
							MountPath: "/var/lib/rabbitmq/mnesia/",
						}))
					})
				})

				Context("Rabbitmq Container EnvVar", func() {
					It("Overrides the envVar list while making sure that 'MY_POD_NAME', 'MY_POD_NAMESPACE' and 'K8S_SERVICE_NAME' are always defined first", func() {
						instance.Spec.Override.StatefulSet = &rabbitmqv1beta1.StatefulSet{
							Spec: &rabbitmqv1beta1.StatefulSetSpec{
								Template: &rabbitmqv1beta1.PodTemplateSpec{
									Spec: &corev1.PodSpec{
										Containers: []corev1.Container{
											{
												Name: "rabbitmq",
												Env: []corev1.EnvVar{
													{
														Name:  "test1",
														Value: "test1",
													},
													{
														Name:  "RABBITMQ_USE_LONGNAME",
														Value: "false",
													},
													{
														Name:  "RABBITMQ_STREAM_ADVERTISED_HOST",
														Value: "$(MY_POD_NAME).$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE)",
													},
												},
											},
										},
									},
								},
							},
						}

						builder = &resource.RabbitmqResourceBuilder{
							Instance: &instance,
							Scheme:   scheme,
						}
						stsBuilder := builder.StatefulSet()
						Expect(stsBuilder.Update(statefulSet)).To(Succeed())
						Expect(extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq").Env[0]).To(Equal(
							corev1.EnvVar{
								Name: "MY_POD_NAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath:  "metadata.name",
										APIVersion: "v1",
									},
								},
							}))
						Expect(extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq").Env[1]).To(Equal(
							corev1.EnvVar{
								Name: "MY_POD_NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath:  "metadata.namespace",
										APIVersion: "v1",
									},
								},
							}))
						Expect(extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq").Env[2]).To(Equal(
							corev1.EnvVar{
								Name:  "K8S_SERVICE_NAME",
								Value: "foo-nodes",
							}))
						Expect(extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq").Env).To(ConsistOf(
							corev1.EnvVar{
								Name: "MY_POD_NAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath:  "metadata.name",
										APIVersion: "v1",
									},
								},
							},
							corev1.EnvVar{
								Name: "MY_POD_NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath:  "metadata.namespace",
										APIVersion: "v1",
									},
								},
							},
							corev1.EnvVar{
								Name:  "test1",
								Value: "test1",
							},
							corev1.EnvVar{
								Name:  "K8S_SERVICE_NAME",
								Value: instance.ChildResourceName("nodes"),
							},
							corev1.EnvVar{
								Name:  "RABBITMQ_USE_LONGNAME",
								Value: "false",
							},
							corev1.EnvVar{
								Name:  "RABBITMQ_ENABLED_PLUGINS_FILE",
								Value: "/operator/enabled_plugins",
							},
							corev1.EnvVar{
								Name:  "RABBITMQ_NODENAME",
								Value: "rabbit@$(MY_POD_NAME).$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE)",
							},
							corev1.EnvVar{
								Name:  "K8S_HOSTNAME_SUFFIX",
								Value: ".$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE)",
							},
							corev1.EnvVar{
								Name:  "RABBITMQ_STREAM_ADVERTISED_HOST",
								Value: "$(MY_POD_NAME).$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE)",
							}))
					})
				})
			})

			It("ensures override takes precedence when same property is set both at the top level and at the override level", func() {
				instance.Spec.Image = "should-be-replaced-image"
				instance.Spec.Replicas = pointer.Int32Ptr(2)

				instance.Spec.Override.StatefulSet = &rabbitmqv1beta1.StatefulSet{
					Spec: &rabbitmqv1beta1.StatefulSetSpec{
						Replicas: pointer.Int32Ptr(4),
						Template: &rabbitmqv1beta1.PodTemplateSpec{
							Spec: &corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "rabbitmq",
										Image: "override-image",
									},
								},
							},
						},
					},
				}

				builder = &resource.RabbitmqResourceBuilder{
					Instance: &instance,
					Scheme:   scheme,
				}
				stsBuilder := builder.StatefulSet()
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())

				Expect(*statefulSet.Spec.Replicas).To(Equal(int32(4)))
				Expect(extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq").Image).To(Equal("override-image"))
			})
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

func generateRabbitmqCluster() rabbitmqv1beta1.RabbitmqCluster {
	storage := k8sresource.MustParse("10Gi")
	return rabbitmqv1beta1.RabbitmqCluster{
		ObjectMeta: v1.ObjectMeta{
			Name:      "foo",
			Namespace: "foo-namespace",
		},
		Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
			Replicas:                      pointer.Int32Ptr(1),
			Image:                         "rabbitmq-image-from-cr",
			ImagePullSecrets:              []corev1.LocalObjectReference{{Name: "my-super-secret"}},
			TerminationGracePeriodSeconds: pointer.Int64Ptr(604800),
			Service: rabbitmqv1beta1.RabbitmqClusterServiceSpec{
				Type:        "this-is-a-service",
				Annotations: map[string]string{},
			},
			Persistence: rabbitmqv1beta1.RabbitmqClusterPersistenceSpec{
				StorageClassName: nil,
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
		},
	}
}
