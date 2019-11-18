package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("StatefulSet", func() {
	var (
		instance                 rabbitmqv1beta1.RabbitmqCluster
		scheme                   *runtime.Scheme
		resourceRequirements     resource.ResourceRequirements
		statefulSetConfiguration resource.StatefulSetConfiguration
		cluster                  *resource.RabbitmqCluster
	)

	Context("when creating a working StatefulSet with default settings", func() {
		var (
			sts      *appsv1.StatefulSet
			instance = rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: v1.ObjectMeta{
					Name:      "foo",
					Namespace: "foo",
				},
			}
		)

		BeforeEach(func() {
			scheme = runtime.NewScheme()
			rabbitmqv1beta1.AddToScheme(scheme)
			defaultscheme.AddToScheme(scheme)

			resourceRequirements = resource.ResourceRequirements{
				CPULimit:      "",
				MemoryLimit:   "",
				CPURequest:    "",
				MemoryRequest: "",
			}

			statefulSetConfiguration = resource.StatefulSetConfiguration{
				ImageReference:              "",
				ImagePullSecret:             "",
				PersistenceStorageClassName: "",
				PersistenceStorage:          "",
				ResourceRequirementsConfig:  resourceRequirements,
				Scheme:                      scheme,
			}
			cluster = &resource.RabbitmqCluster{
				Instance:                 &instance,
				StatefulSetConfiguration: statefulSetConfiguration,
			}
			sts, _ = cluster.StatefulSet()
		})

		It("sets the right service name", func() {
			Expect(sts.Spec.ServiceName).To(Equal(instance.ChildResourceName("headless")))
		})

		It("sets replicas to be '1' by default", func() {
			Expect(*sts.Spec.Replicas).To(Equal(int32(1)))
		})

		It("adds the correct labels on the statefulset", func() {
			labels := sts.Labels
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
			Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
		})

		It("adds the correct name with naming conventions", func() {
			expectedName := instance.ChildResourceName("server")
			Expect(sts.Name).To(Equal(expectedName))
		})

		It("specifies required Container Ports", func() {
			requiredContainerPorts := []int32{4369, 5672, 15672, 15692}
			var actualContainerPorts []int32

			container := extractContainer(sts.Spec.Template.Spec.Containers, "rabbitmq")
			for _, port := range container.Ports {
				actualContainerPorts = append(actualContainerPorts, port.ContainerPort)
			}

			Expect(actualContainerPorts).Should(ConsistOf(requiredContainerPorts))
		})

		It("uses required Environment Variables", func() {
			requiredEnvVariables := []corev1.EnvVar{
				{
					Name:  "RABBITMQ_ENABLED_PLUGINS_FILE",
					Value: "/opt/server-conf/enabled_plugins",
				},
				{
					Name:  "RABBITMQ_DEFAULT_PASS_FILE",
					Value: "/opt/rabbitmq-secret/rabbitmq-password",
				},
				{
					Name:  "RABBITMQ_DEFAULT_USER_FILE",
					Value: "/opt/rabbitmq-secret/rabbitmq-username",
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

			container := extractContainer(sts.Spec.Template.Spec.Containers, "rabbitmq")
			Expect(container.Env).Should(ConsistOf(requiredEnvVariables))
		})

		It("creates required Volume Mounts for the rabbitmq container", func() {
			container := extractContainer(sts.Spec.Template.Spec.Containers, "rabbitmq")
			Expect(container.VolumeMounts).Should(ConsistOf(
				corev1.VolumeMount{
					Name:      "server-conf",
					MountPath: "/opt/server-conf/",
				},
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
			))
		})

		It("defines the expected volumes", func() {
			Expect(sts.Spec.Template.Spec.Volumes).Should(ConsistOf(
				corev1.Volume{
					Name: "rabbitmq-admin",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: instance.ChildResourceName("admin"),
							Items: []corev1.KeyToPath{
								{
									Key:  "rabbitmq-username",
									Path: "rabbitmq-username",
								},
								{
									Key:  "rabbitmq-password",
									Path: "rabbitmq-password",
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
			))
		})

		It("uses the correct service account", func() {
			Expect(sts.Spec.Template.Spec.ServiceAccountName).To(Equal(instance.ChildResourceName("server")))
		})

		It("does mount the service account in its pods", func() {
			Expect(*sts.Spec.Template.Spec.AutomountServiceAccountToken).To(BeTrue())
		})

		It("creates the required PersistentVolumeClaim", func() {
			truth := true
			q, _ := k8sresource.ParseQuantity("10Gi")

			expectedPersistentVolumeClaim := corev1.PersistentVolumeClaim{
				ObjectMeta: v1.ObjectMeta{
					Name: "persistence",
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

			actualPersistentVolumeClaim := sts.Spec.VolumeClaimTemplates[0]
			Expect(actualPersistentVolumeClaim).To(Equal(expectedPersistentVolumeClaim))
		})

		It("creates the required SecurityContext", func() {
			rmqGID, rmqUID := int64(999), int64(999)

			expectedPodSecurityContext := &corev1.PodSecurityContext{
				FSGroup:    &rmqGID,
				RunAsGroup: &rmqGID,
				RunAsUser:  &rmqUID,
			}

			Expect(sts.Spec.Template.Spec.SecurityContext).To(Equal(expectedPodSecurityContext))
		})

		It("defines a Readiness Probe", func() {
			container := extractContainer(sts.Spec.Template.Spec.Containers, "rabbitmq")
			actualProbeCommand := container.ReadinessProbe.Handler.Exec.Command
			Expect(actualProbeCommand).To(Equal([]string{"/bin/sh", "-c", "rabbitmq-diagnostics check_running && rabbitmq-diagnostics check_port_connectivity"}))
		})

		It("templates the image string and the imagePullSecrets with default values", func() {
			container := extractContainer(sts.Spec.Template.Spec.Containers, "rabbitmq")
			Expect(container.Image).To(Equal("rabbitmq:3.8.1"))
			Expect(sts.Spec.Template.Spec.ImagePullSecrets).To(BeEmpty())
		})

		It("templates the correct InitContainer", func() {
			initContainers := sts.Spec.Template.Spec.InitContainers
			Expect(len(initContainers)).To(Equal(1))

			container := extractContainer(initContainers, "copy-config")
			Expect(container.Command).To(Equal([]string{
				"sh", "-c", "cp /tmp/rabbitmq/rabbitmq.conf /etc/rabbitmq/rabbitmq.conf && echo '' >> /etc/rabbitmq/rabbitmq.conf ; " +
					"cp /tmp/erlang-cookie-secret/.erlang.cookie /var/lib/rabbitmq/.erlang.cookie " +
					"&& chown 999:999 /var/lib/rabbitmq/.erlang.cookie " +
					"&& chmod 600 /var/lib/rabbitmq/.erlang.cookie",
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

			Expect(container.Image).To(Equal("rabbitmq:3.8.1"))
		})

		It("templates the correct resource limits for the Rabbitmq container", func() {
			container := extractContainer(sts.Spec.Template.Spec.Containers, "rabbitmq")

			cpuLimit, err := k8sresource.ParseQuantity("500m")
			Expect(err).NotTo(HaveOccurred())
			memoryLimit, err := k8sresource.ParseQuantity("2Gi")
			Expect(err).NotTo(HaveOccurred())

			expectedResourceLimits := corev1.ResourceList{
				corev1.ResourceCPU:    cpuLimit,
				corev1.ResourceMemory: memoryLimit,
			}
			Expect(container.Resources.Limits).To(Equal(expectedResourceLimits))
		})

		It("templates the correct resource requests for the Rabbitmq container", func() {
			container := extractContainer(sts.Spec.Template.Spec.Containers, "rabbitmq")

			cpuRequest, err := k8sresource.ParseQuantity("100m")
			Expect(err).NotTo(HaveOccurred())
			memoryRequest, err := k8sresource.ParseQuantity("2Gi")
			Expect(err).NotTo(HaveOccurred())

			expectResourceRequests := corev1.ResourceList{
				corev1.ResourceCPU:    cpuRequest,
				corev1.ResourceMemory: memoryRequest,
			}
			Expect(container.Resources.Requests).To(Equal(expectResourceRequests))
		})

		It("adds the correct labels on the rabbitmq pods", func() {
			labels := sts.Spec.Template.ObjectMeta.Labels
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
			Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
		})

		It("adds the correct label selector", func() {
			labels := sts.Spec.Selector.MatchLabels
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
		})
	})

	Context("when creating a working StatefulSet with non-default settings", func() {
		BeforeEach(func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{}
			instance.Namespace = "foo"
			instance.Name = "foo"
			scheme = runtime.NewScheme()
			rabbitmqv1beta1.AddToScheme(scheme)
			defaultscheme.AddToScheme(scheme)
		})

		When("storage class name is specified in both as parameters and in RabbitmqCluster instance", func() {
			BeforeEach(func() {
				instance.Spec.Persistence.StorageClassName = "my-storage-class"
				statefulSetConfiguration.PersistenceStorageClassName = "a-storage-class-name"
			})
			It("creates the PersistentVolume template according to configurations in the RabbitmqCluster instance", func() {
				cluster = &resource.RabbitmqCluster{
					Instance:                 &instance,
					StatefulSetConfiguration: statefulSetConfiguration,
				}
				statefulSet, _ := cluster.StatefulSet()
				Expect(*statefulSet.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(Equal("my-storage-class"))
			})
		})

		When("storage class name is specified only as parameters and not in RabbitmqCluster instance", func() {
			BeforeEach(func() {
				instance.Spec.Persistence.StorageClassName = ""
				statefulSetConfiguration.PersistenceStorageClassName = "a-storage-class-name"
			})
			It("creates the PersistentVolume template according to the parameters", func() {
				cluster = &resource.RabbitmqCluster{
					Instance:                 &instance,
					StatefulSetConfiguration: statefulSetConfiguration,
				}
				statefulSet, _ := cluster.StatefulSet()
				Expect(*statefulSet.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(Equal("a-storage-class-name"))
			})
		})

		When("storage class name is empty in parameters and is empty in RabbitmqCluster instance", func() {
			BeforeEach(func() {
				instance.Spec.Persistence.StorageClassName = ""
				statefulSetConfiguration.PersistenceStorageClassName = ""
			})
			It("creates the PersistentVolume template with empty class so it defaults to  default StorageClass", func() {
				cluster = &resource.RabbitmqCluster{
					Instance:                 &instance,
					StatefulSetConfiguration: statefulSetConfiguration,
				}
				statefulSet, _ := cluster.StatefulSet()
				Expect(statefulSet.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(BeNil())
			})
		})

		When("storage class capacity is specified in both as parameters and in RabbitmqCluster instance", func() {
			BeforeEach(func() {
				instance.Spec.Persistence.Storage = "21Gi"
				statefulSetConfiguration.PersistenceStorage = "100Gi"
			})
			It("creates the PersistentVolume template according to configurations in the RabbitmqCluster instance", func() {
				cluster = &resource.RabbitmqCluster{
					Instance:                 &instance,
					StatefulSetConfiguration: statefulSetConfiguration,
				}
				statefulSet, _ := cluster.StatefulSet()
				q, _ := k8sresource.ParseQuantity("21Gi")
				Expect(statefulSet.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests["storage"]).To(Equal(q))
			})
		})

		When("storage capacity is specified only as parameters and not in RabbitmqCluster instance", func() {
			BeforeEach(func() {
				instance.Spec.Persistence.Storage = ""
				statefulSetConfiguration.PersistenceStorage = "100Gi"
			})
			It("creates the PersistentVolume template according to the parameters", func() {
				cluster = &resource.RabbitmqCluster{
					Instance:                 &instance,
					StatefulSetConfiguration: statefulSetConfiguration,
				}
				statefulSet, _ := cluster.StatefulSet()
				q, _ := k8sresource.ParseQuantity("100Gi")
				Expect(statefulSet.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests["storage"]).To(Equal(q))
			})
		})

		When("storage capacity is empty in parameters and is empty in RabbitmqCluster instance", func() {
			BeforeEach(func() {
				instance.Spec.Persistence.StorageClassName = ""
				statefulSetConfiguration.PersistenceStorage = ""
			})
			It("creates the PersistentVolume template with default capacity", func() {
				cluster = &resource.RabbitmqCluster{
					Instance:                 &instance,
					StatefulSetConfiguration: statefulSetConfiguration,
				}
				statefulSet, _ := cluster.StatefulSet()
				q, _ := k8sresource.ParseQuantity("10Gi")
				Expect(statefulSet.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests["storage"]).To(Equal(q))
			})
		})

		When("configuring a private image repository", func() {
			It("templates the image string and the imagePullSecrets correctly", func() {
				instance.Spec.Image = "my-private-repo/rabbitmq:3.8.0"
				instance.Spec.ImagePullSecret = "my-great-secret"

				cluster = &resource.RabbitmqCluster{
					Instance:                 &instance,
					StatefulSetConfiguration: statefulSetConfiguration,
				}
				statefulSet, _ := cluster.StatefulSet()
				container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(container.Image).To(Equal("my-private-repo/rabbitmq:3.8.0"))
				Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "my-great-secret"}))

				initContainer := extractContainer(statefulSet.Spec.Template.Spec.InitContainers, "copy-config")
				Expect(initContainer.Image).To(Equal("my-private-repo/rabbitmq:3.8.0"))
			})
		})

		When("providing resources limits and requests", func() {
			Context("CPU and memory limit are provided", func() {
				BeforeEach(func() {
					statefulSetConfiguration.ResourceRequirementsConfig = resource.ResourceRequirements{
						CPULimit:      "1m",
						MemoryLimit:   "10Gi",
						CPURequest:    "",
						MemoryRequest: "",
					}
				})

				It("generates a statefulSet with provided CPU and memory limits, with default CPU and memory requests", func() {
					cluster = &resource.RabbitmqCluster{
						Instance:                 &instance,
						StatefulSetConfiguration: statefulSetConfiguration,
					}
					statefulSet, _ := cluster.StatefulSet()
					expectedCPULimit, _ := k8sresource.ParseQuantity("1m")
					expectedMemoryLimit, _ := k8sresource.ParseQuantity("10Gi")
					defaultCPURequest, _ := k8sresource.ParseQuantity("100m")
					defaultMemoryRequest, _ := k8sresource.ParseQuantity("2Gi")

					container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
					Expect(container.Resources.Limits[corev1.ResourceCPU]).To(Equal(expectedCPULimit))
					Expect(container.Resources.Requests[corev1.ResourceCPU]).To(Equal(defaultCPURequest))

					Expect(container.Resources.Limits[corev1.ResourceMemory]).To(Equal(expectedMemoryLimit))
					Expect(container.Resources.Requests[corev1.ResourceMemory]).To(Equal(defaultMemoryRequest))
				})
			})

			Context("CPU and memory requests are provided", func() {
				BeforeEach(func() {
					statefulSetConfiguration.ResourceRequirementsConfig = resource.ResourceRequirements{
						CPULimit:      "",
						MemoryLimit:   "",
						CPURequest:    "10m",
						MemoryRequest: "5Gi",
					}
				})
				It("generates a statefulSet with provided CPU and memory requests, with default CPU and memory limits", func() {
					cluster = &resource.RabbitmqCluster{
						Instance:                 &instance,
						StatefulSetConfiguration: statefulSetConfiguration,
					}
					statefulSet, _ := cluster.StatefulSet()
					expectedCPURequest, _ := k8sresource.ParseQuantity("10m")
					expectedMemoryRequest, _ := k8sresource.ParseQuantity("5Gi")
					defaultCPULimit, _ := k8sresource.ParseQuantity("500m")
					defaultMemoryLimit, _ := k8sresource.ParseQuantity("2Gi")

					container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
					Expect(container.Resources.Limits[corev1.ResourceMemory]).To(Equal(defaultMemoryLimit))
					Expect(container.Resources.Requests[corev1.ResourceMemory]).To(Equal(expectedMemoryRequest))

					Expect(container.Resources.Limits[corev1.ResourceCPU]).To(Equal(defaultCPULimit))
					Expect(container.Resources.Requests[corev1.ResourceCPU]).To(Equal(expectedCPURequest))
				})
			})

			Context("both CPU and memory's limits plus requests are provided", func() {
				BeforeEach(func() {
					statefulSetConfiguration.ResourceRequirementsConfig = resource.ResourceRequirements{
						CPULimit:      "10m",
						MemoryLimit:   "5Gi",
						CPURequest:    "1m",
						MemoryRequest: "1Gi",
					}
				})

				It("generates a statefulSet with provided memory limit/request and provided CPU limit/request", func() {
					cluster = &resource.RabbitmqCluster{
						Instance:                 &instance,
						StatefulSetConfiguration: statefulSetConfiguration,
					}
					statefulSet, _ := cluster.StatefulSet()
					expectedCPULimit, _ := k8sresource.ParseQuantity("10m")
					expectedCPURequest, _ := k8sresource.ParseQuantity("1m")
					expectedMemoryLimit, _ := k8sresource.ParseQuantity("5Gi")
					expectedMemoryRequest, _ := k8sresource.ParseQuantity("1Gi")

					container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
					Expect(container.Resources.Limits[corev1.ResourceMemory]).To(Equal(expectedMemoryLimit))
					Expect(container.Resources.Requests[corev1.ResourceMemory]).To(Equal(expectedMemoryRequest))

					Expect(container.Resources.Limits[corev1.ResourceCPU]).To(Equal(expectedCPULimit))
					Expect(container.Resources.Requests[corev1.ResourceCPU]).To(Equal(expectedCPURequest))
				})
			})
		})

		When("image repository and ImagePullSecret are provided through function params", func() {

			// Anonymouse function used in this context because we had issues scoping the instance and scheme without a closure
			It("uses the provided repository and secret if not specified in RabbitmqCluster spec", func() {
				statefulSetConfiguration.ImageReference = "best-repository/rabbitmq:some-tag"
				statefulSetConfiguration.ImagePullSecret = "my-secret"
				cluster = &resource.RabbitmqCluster{
					Instance:                 &instance,
					StatefulSetConfiguration: statefulSetConfiguration,
				}
				statefulSet, _ := cluster.StatefulSet()
				container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(container.Image).To(Equal("best-repository/rabbitmq:some-tag"))
				Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "my-secret"}))
			})

			It("uses the RabbitmqCluster spec if it is provided", func() {
				instance.Spec.Image = "my-private-repo/rabbitmq:latest"
				instance.Spec.ImagePullSecret = "my-great-secret"
				statefulSetConfiguration.ImageReference = "best-repository/rabbitmq:some-tag"
				statefulSetConfiguration.ImagePullSecret = "my-secret"
				cluster = &resource.RabbitmqCluster{
					Instance:                 &instance,
					StatefulSetConfiguration: statefulSetConfiguration,
				}
				statefulSet, _ := cluster.StatefulSet()
				container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(container.Image).To(Equal("my-private-repo/rabbitmq:latest"))
				Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "my-great-secret"}))
			})
		})

		When("replica count is specified in the RabbitmqCluster spec", func() {
			It("sets the replica count of the StatefulSet to the provided value", func() {
				instance.Spec.Replicas = 3
				cluster = &resource.RabbitmqCluster{
					Instance:                 &instance,
					StatefulSetConfiguration: statefulSetConfiguration,
				}
				statefulSet, _ := cluster.StatefulSet()
				Expect(*statefulSet.Spec.Replicas).To(Equal(int32(3)))
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
