package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("StatefulSet", func() {
	var (
		instance   rabbitmqv1beta1.RabbitmqCluster
		scheme     *runtime.Scheme
		cluster    *resource.RabbitmqResourceBuilder
		stsBuilder *resource.StatefulSetBuilder
	)

	Context("Build", func() {
		BeforeEach(func() {
			instance = generateRabbitmqCluster()

			scheme = runtime.NewScheme()
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
			Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())
			cluster = &resource.RabbitmqResourceBuilder{
				Instance: &instance,
				Scheme:   scheme,
			}

			stsBuilder = cluster.StatefulSet(resource.MockClusterDomain("cluster.local"))
		})

		It("sets the name and namespace", func() {
			obj, err := stsBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			sts := obj.(*appsv1.StatefulSet)

			Expect(sts.Name).To(Equal("foo-rabbitmq-server"))
			Expect(sts.Namespace).To(Equal("foo-namespace"))
		})

		It("sets the right service name", func() {
			obj, err := stsBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			statefulSet := obj.(*appsv1.StatefulSet)

			Expect(statefulSet.Spec.ServiceName).To(Equal(instance.ChildResourceName("headless")))
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
			instance.Spec.Persistence.StorageClassName = &storageClassName
			cluster = &resource.RabbitmqResourceBuilder{
				Instance: &instance,
				Scheme:   scheme,
			}
			stsBuilder := cluster.StatefulSet(resource.MockClusterDomain("cluster.local"))
			obj, err := stsBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			statefulSet := obj.(*appsv1.StatefulSet)

			Expect(*statefulSet.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(Equal("my-storage-class"))
		})

		It("creates the PersistentVolume template according to configurations in the  instance", func() {
			storage := k8sresource.MustParse("21Gi")
			instance.Spec.Persistence.Storage = &storage
			cluster = &resource.RabbitmqResourceBuilder{
				Instance: &instance,
				Scheme:   scheme,
			}
			stsBuilder := cluster.StatefulSet(resource.MockClusterDomain("cluster.local"))
			obj, err := stsBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			statefulSet := obj.(*appsv1.StatefulSet)

			q, _ := k8sresource.ParseQuantity("21Gi")
			Expect(statefulSet.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests["storage"]).To(Equal(q))
		})
		Context("PVC template", func() {
			It("creates the required PersistentVolumeClaim", func() {
				truth := true
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
							"app.kubernetes.io/part-of":   "pivotal-rabbitmq",
						},
						OwnerReferences: []v1.OwnerReference{
							{
								APIVersion:         "rabbitmq.pivotal.io/v1beta1",
								Kind:               "RabbitmqCluster",
								Name:               instance.Name,
								UID:                "",
								Controller:         &truth,
								BlockOwnerDeletion: &truth,
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
	})

	Context("Update", func() {
		var (
			statefulSet *appsv1.StatefulSet
			stsBuilder  *resource.StatefulSetBuilder
			affinity    *corev1.Affinity
		)

		BeforeEach(func() {
			instance = generateRabbitmqCluster()

			scheme = runtime.NewScheme()
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
			Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())

			cluster = &resource.RabbitmqResourceBuilder{
				Instance: &instance,
				Scheme:   scheme,
			}

			stsBuilder = cluster.StatefulSet(resource.MockClusterDomain("cluster.local"))

			statefulSet = &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instance.Name,
					Namespace: instance.Namespace,
				},
			}
		})

		It("creates the affinity rule as provided in the instance", func() {
			affinity = &corev1.Affinity{
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
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			Expect(len(statefulSet.OwnerReferences)).To(Equal(1))
			Expect(statefulSet.OwnerReferences[0].Name).To(Equal(cluster.Instance.Name))
		})

		It("specifies the upgrade policy", func() {
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())
			zero := int32(0)
			updateStrategy := appsv1.StatefulSetUpdateStrategy{
				RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
					Partition: &zero,
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

				cluster = &resource.RabbitmqResourceBuilder{
					Instance: &instance,
					Scheme:   scheme,
				}

				stsBuilder = cluster.StatefulSet(resource.MockClusterDomain("cluster.local"))
			})

			It("restores the default labels", func() {
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())

				labels := statefulSet.Spec.Template.Labels
				Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
				Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
				Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
			})

			It("deletes the labels that are removed from the CR", func() {
				existingLabels := map[string]string{
					"app.kubernetes.io/name":      instance.Name,
					"app.kubernetes.io/part-of":   "pivotal-rabbitmq",
					"this-was-the-previous-label": "should-be-deleted",
				}

				statefulSet.Labels = existingLabels
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())

				Expect(statefulSet.Spec.Template.Labels).NotTo(HaveKey("this-was-the-previous-label"))
			})

			It("has the labels from the instance on the statefulset", func() {
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())
				testLabels(statefulSet.Labels)
			})

			It("has the labels from the instance on the pod", func() {
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())
				podTemplate := statefulSet.Spec.Template
				testLabels(podTemplate.Labels)
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
					"app.kubernetes.io/part-of":   "pivotal-rabbitmq",
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
				Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))

				By("deleting the labels that are removed from the CR")
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())
				Expect(statefulSet.Labels).NotTo(HaveKey("this-was-the-previous-label"))
			})

			It("adds the correct labels on the rabbitmq pods", func() {
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())

				labels := statefulSet.Spec.Template.ObjectMeta.Labels
				Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
				Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
				Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
			})
		})

		Context("annotations", func() {
			var (
				existingAnnotations            map[string]string
				existingPodTemplateAnnotations map[string]string
			)

			BeforeEach(func() {
				existingAnnotations = map[string]string{
					"this-was-the-previous-annotation": "should-be-preserved",
					"app.kubernetes.io/part-of":        "pivotal-rabbitmq",
					"app.k8s.io/something":             "something-amazing",
				}

				existingPodTemplateAnnotations = map[string]string{
					"this-was-the-previous-pod-anno": "should-be-preserved",
					"app.kubernetes.io/part-of":      "pivotal-rabbitmq-pod",
					"app.k8s.io/something":           "something-amazing-on-pod",
				}

				statefulSet.Annotations = existingAnnotations
				statefulSet.Spec.Template.Annotations = existingPodTemplateAnnotations
			})

			It("updates annotations", func() {
				stsBuilder.Instance.Annotations = map[string]string{
					"my-annotation":              "i-like-this",
					"kubernetes.io/name":         "i-do-not-like-this",
					"kubectl.kubernetes.io/name": "i-do-not-like-this",
					"k8s.io/name":                "i-do-not-like-this",
				}
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())

				expectedAnnotations := map[string]string{
					"my-annotation":                    "i-like-this",
					"this-was-the-previous-annotation": "should-be-preserved",
					"app.kubernetes.io/part-of":        "pivotal-rabbitmq",
					"app.k8s.io/something":             "something-amazing",
				}

				Expect(statefulSet.Annotations).To(Equal(expectedAnnotations))
			})

			It("update annotations from the instance to the pod", func() {
				stsBuilder.Instance.Annotations = map[string]string{
					"my-annotation":              "i-like-this",
					"kubernetes.io/name":         "i-do-not-like-this",
					"kubectl.kubernetes.io/name": "i-do-not-like-this",
					"k8s.io/name":                "i-do-not-like-this",
				}

				Expect(stsBuilder.Update(statefulSet)).To(Succeed())
				expectedAnnotations := map[string]string{
					"my-annotation":                  "i-like-this",
					"app.kubernetes.io/part-of":      "pivotal-rabbitmq-pod",
					"this-was-the-previous-pod-anno": "should-be-preserved",
					"app.k8s.io/something":           "something-amazing-on-pod",
				}

				Expect(statefulSet.Spec.Template.Annotations).To(Equal(expectedAnnotations))
			})
		})

		It("updates the image pull secret; sets it back to default after deleting the configuration", func() {
			stsBuilder.Instance.Spec.ImagePullSecret = "my-shiny-new-secret"
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())
			Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "my-shiny-new-secret"}))

			stsBuilder.Instance.Spec.ImagePullSecret = ""
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())
			Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(BeEmpty())
		})

		It("sets replicas", func() {
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			Expect(*statefulSet.Spec.Replicas).To(Equal(int32(1)))
		})

		It("has resources requirements on the init container", func() {
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			resources := statefulSet.Spec.Template.Spec.InitContainers[0].Resources
			Expect(resources.Requests["cpu"]).To(Equal(k8sresource.MustParse("100m")))
			Expect(resources.Requests["memory"]).To(Equal(k8sresource.MustParse("500Mi")))
			Expect(resources.Limits["cpu"]).To(Equal(k8sresource.MustParse("100m")))
			Expect(resources.Limits["memory"]).To(Equal(k8sresource.MustParse("500Mi")))
		})

		It("specifies required Container Ports", func() {
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			requiredContainerPorts := []int32{4369, 5672, 15672, 15692}
			var actualContainerPorts []int32

			container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
			for _, port := range container.Ports {
				actualContainerPorts = append(actualContainerPorts, port.ContainerPort)
			}

			Expect(actualContainerPorts).Should(ConsistOf(requiredContainerPorts))
		})

		It("uses required Environment Variables", func() {
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			requiredEnvVariables := []corev1.EnvVar{
				{
					Name:  "RABBITMQ_DEFAULT_PASS_FILE",
					Value: "/opt/rabbitmq-secret/password",
				},
				{
					Name:  "RABBITMQ_DEFAULT_USER_FILE",
					Value: "/opt/rabbitmq-secret/username",
				},
				{
					Name:  "RABBITMQ_MNESIA_BASE",
					Value: "/var/lib/rabbitmq/db",
				},
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
					Name:  "K8S_SERVICE_NAME",
					Value: instance.ChildResourceName("headless"),
				},
				{
					Name:  "RABBITMQ_USE_LONGNAME",
					Value: "true",
				},
				{
					Name:  "RABBITMQ_NODENAME",
					Value: "rabbit@$(MY_POD_NAME).$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE).svc.cluster.local",
				},
				{
					Name:  "K8S_HOSTNAME_SUFFIX",
					Value: ".$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE).svc.cluster.local",
				},
			}

			container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
			Expect(container.Env).Should(ConsistOf(requiredEnvVariables))
		})

		It("creates required Volume Mounts for the rabbitmq container", func() {
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
			Expect(container.VolumeMounts).Should(ConsistOf(
				corev1.VolumeMount{
					Name:      "rabbitmq-admin",
					MountPath: "/opt/rabbitmq-secret/",
				},
				corev1.VolumeMount{
					Name:      "persistence",
					MountPath: "/var/lib/rabbitmq/db/",
				},
				corev1.VolumeMount{
					Name:      "rabbitmq-etc",
					MountPath: "/etc/rabbitmq/",
				},
				corev1.VolumeMount{
					Name:      "rabbitmq-erlang-cookie",
					MountPath: "/var/lib/rabbitmq/",
				},
				corev1.VolumeMount{
					Name:      "pod-info",
					MountPath: "/etc/pod-info/",
				},
			))
		})

		It("defines the expected volumes", func() {
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			Expect(statefulSet.Spec.Template.Spec.Volumes).Should(ConsistOf(
				corev1.Volume{
					Name: "rabbitmq-admin",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: instance.ChildResourceName("admin"),
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
							LocalObjectReference: corev1.LocalObjectReference{
								Name: instance.ChildResourceName("server-conf"),
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
							SecretName: instance.ChildResourceName("erlang-cookie"),
						},
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
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			Expect(statefulSet.Spec.Template.Spec.ServiceAccountName).To(Equal(instance.ChildResourceName("server")))
		})

		It("mounts the service account in its pods", func() {
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			Expect(*statefulSet.Spec.Template.Spec.AutomountServiceAccountToken).To(BeTrue())
		})

		It("creates the required SecurityContext", func() {
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
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
			actualProbeCommand := container.ReadinessProbe.Handler.Exec.Command
			Expect(actualProbeCommand).To(Equal([]string{"/bin/sh", "-c", "rabbitmq-diagnostics check_port_connectivity"}))
		})

		It("templates the correct InitContainer", func() {
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			initContainers := statefulSet.Spec.Template.Spec.InitContainers
			Expect(len(initContainers)).To(Equal(1))

			container := extractContainer(initContainers, "copy-config")
			Expect(container.Command).To(Equal([]string{
				"sh", "-c", "cp /tmp/rabbitmq/rabbitmq.conf /etc/rabbitmq/rabbitmq.conf && echo '' >> /etc/rabbitmq/rabbitmq.conf ; " +
					"cp /tmp/erlang-cookie-secret/.erlang.cookie /var/lib/rabbitmq/.erlang.cookie " +
					"&& chown 999:999 /var/lib/rabbitmq/.erlang.cookie " +
					"&& chmod 600 /var/lib/rabbitmq/.erlang.cookie ; " +
					"cp /tmp/rabbitmq/enabled_plugins /etc/rabbitmq/enabled_plugins " +
					"&& chown 999:999 /etc/rabbitmq/enabled_plugins",
			}))

			Expect(container.VolumeMounts).Should(ConsistOf(
				corev1.VolumeMount{
					Name:      "server-conf",
					MountPath: "/tmp/rabbitmq/",
				},
				corev1.VolumeMount{
					Name:      "rabbitmq-etc",
					MountPath: "/etc/rabbitmq/",
				},
				corev1.VolumeMount{
					Name:      "rabbitmq-erlang-cookie",
					MountPath: "/var/lib/rabbitmq/",
				},
				corev1.VolumeMount{
					Name:      "erlang-cookie-secret",
					MountPath: "/tmp/erlang-cookie-secret/",
				},
			))

			Expect(container.Image).To(Equal("rabbitmq-image-from-cr"))
		})

		It("adds the required terminationGracePeriodSeconds", func() {
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			gracePeriodSeconds := statefulSet.Spec.Template.Spec.TerminationGracePeriodSeconds
			expectedGracePeriodSeconds := int64(60 * 60 * 24 * 7)
			Expect(gracePeriodSeconds).To(Equal(&expectedGracePeriodSeconds))
		})

		It("checks mirror and querum queue status in preStop hook", func() {
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			expectedPreStopCommand := []string{"/bin/bash", "-c", "if [ ! -z \"$(cat /etc/pod-info/skipPreStopChecks)\" ]; then exit 0; fi; while true; do rabbitmq-queues check_if_node_is_quorum_critical 2>&1; if [ $(echo $?) -eq 69 ]; then sleep 2; continue; fi; rabbitmq-queues check_if_node_is_mirror_sync_critical 2>&1; if [ $(echo $?) -eq 69 ]; then sleep 2; continue; fi; break; done"}

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

				cluster = &resource.RabbitmqResourceBuilder{
					Instance: &instance,
					Scheme:   scheme,
				}

				stsBuilder := cluster.StatefulSet(resource.MockClusterDomain("cluster.local"))
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

				cluster = &resource.RabbitmqResourceBuilder{
					Instance: &instance,
					Scheme:   scheme,
				}

				stsBuilder := cluster.StatefulSet(resource.MockClusterDomain("cluster.local"))
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())

				container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(len(container.Resources.Requests)).To(Equal(0))
				Expect(len(container.Resources.Limits)).To(Equal(0))
			})
		})

		When("configures private image", func() {
			It("uses the instance ImagePullSecret and image reference when provided", func() {
				instance.Spec.Image = "my-private-repo/rabbitmq:latest"
				instance.Spec.ImagePullSecret = "my-great-secret"
				cluster = &resource.RabbitmqResourceBuilder{
					Instance: &instance,
					Scheme:   scheme,
				}

				stsBuilder := cluster.StatefulSet(resource.MockClusterDomain("cluster.local"))
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())
				container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(container.Image).To(Equal("my-private-repo/rabbitmq:latest"))
				Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "my-great-secret"}))
			})
		})

		When("the cluster domain is not the default `cluster.local`", func() {
			It("sets the cluster domain correctly", func() {
				stsBuilder := cluster.StatefulSet(resource.MockClusterDomain("foo.bar"))
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())

				expectedEnvVariables := []corev1.EnvVar{
					{
						Name:  "RABBITMQ_NODENAME",
						Value: "rabbit@$(MY_POD_NAME).$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE).svc.foo.bar",
					},
					{
						Name:  "K8S_HOSTNAME_SUFFIX",
						Value: ".$(K8S_SERVICE_NAME).$(MY_POD_NAMESPACE).svc.foo.bar",
					},
				}

				container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(container.Env).Should(ContainElements(expectedEnvVariables))
			})
		})

		It("sets the replica count of the StatefulSet to the instance value", func() {
			instance.Spec.Replicas = 3
			cluster = &resource.RabbitmqResourceBuilder{
				Instance: &instance,
				Scheme:   scheme,
			}
			stsBuilder := cluster.StatefulSet(resource.MockClusterDomain("cluster.local"))
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())
			Expect(*statefulSet.Spec.Replicas).To(Equal(int32(3)))
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
			Replicas:        int32(1),
			Image:           "rabbitmq-image-from-cr",
			ImagePullSecret: "my-super-secret",
			Service: rabbitmqv1beta1.RabbitmqClusterServiceSpec{
				Type:        corev1.ServiceType("this-is-a-service"),
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
							corev1.NodeSelectorTerm{
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
				corev1.Toleration{
					Key:      "mykey",
					Operator: "NotEqual",
					Value:    "myvalue",
					Effect:   "NoSchedule",
				},
			},
		},
	}
}
