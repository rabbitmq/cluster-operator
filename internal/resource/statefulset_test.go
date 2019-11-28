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
		instance             rabbitmqv1beta1.RabbitmqCluster
		scheme               *runtime.Scheme
		resourceRequirements resource.ResourceRequirements
		defaultConfiguration resource.DefaultConfiguration
		cluster              *resource.RabbitmqResourceBuilder
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
				Limit: resource.ComputeResource{
					CPU:    "",
					Memory: "",
				},
				Request: resource.ComputeResource{
					CPU:    "",
					Memory: "",
				},
			}

			defaultConfiguration = resource.DefaultConfiguration{
				ImageReference:             "",
				ImagePullSecret:            "",
				PersistentStorageClassName: "",
				PersistentStorage:          "",
				ResourceRequirements:       resourceRequirements,
				Scheme:                     scheme,
			}
			cluster = &resource.RabbitmqResourceBuilder{
				Instance:             &instance,
				DefaultConfiguration: defaultConfiguration,
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

		It("has resources requirements on the init container", func() {
			resources := sts.Spec.Template.Spec.InitContainers[0].Resources
			Expect(resources.Requests["cpu"]).To(Equal(k8sresource.MustParse("100m")))
			Expect(resources.Requests["memory"]).To(Equal(k8sresource.MustParse("500Mi")))
			Expect(resources.Limits["cpu"]).To(Equal(k8sresource.MustParse("100m")))
			Expect(resources.Limits["memory"]).To(Equal(k8sresource.MustParse("500Mi")))
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
			Expect(actualProbeCommand).To(Equal([]string{"/bin/sh", "-c", "rabbitmq-diagnostics check_port_connectivity"}))
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

			cpuLimit, err := k8sresource.ParseQuantity(defaultCPULimit)
			Expect(err).NotTo(HaveOccurred())
			memoryLimit, err := k8sresource.ParseQuantity(defaultMemoryLimit)
			Expect(err).NotTo(HaveOccurred())

			expectedResourceLimits := corev1.ResourceList{
				corev1.ResourceCPU:    cpuLimit,
				corev1.ResourceMemory: memoryLimit,
			}
			Expect(container.Resources.Limits).To(Equal(expectedResourceLimits))
		})

		It("templates the correct resource requests for the Rabbitmq container", func() {
			container := extractContainer(sts.Spec.Template.Spec.Containers, "rabbitmq")

			cpuRequest, err := k8sresource.ParseQuantity(defaultCPURequest)
			Expect(err).NotTo(HaveOccurred())
			memoryRequest, err := k8sresource.ParseQuantity(defaultMemoryRequest)
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

		It("adds the required terminationGracePeriodSeconds", func() {
			gracePeriodSeconds := sts.Spec.Template.Spec.TerminationGracePeriodSeconds
			expectedGracePeriodSeconds := int64(150)
			Expect(gracePeriodSeconds).To(Equal(&expectedGracePeriodSeconds))
		})
	})

	Context("when creating a StatefulSet with non-default settings", func() {
		BeforeEach(func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{}
			instance.Namespace = "foo"
			instance.Name = "foo"

			scheme = runtime.NewScheme()
			rabbitmqv1beta1.AddToScheme(scheme)
			defaultscheme.AddToScheme(scheme)

			resourceRequirements = resource.ResourceRequirements{
				Limit: resource.ComputeResource{
					CPU:    "",
					Memory: "",
				},
				Request: resource.ComputeResource{
					CPU:    "",
					Memory: "",
				},
			}

			defaultConfiguration = resource.DefaultConfiguration{
				ResourceRequirements: resourceRequirements,
				Scheme:               scheme,
			}
		})

		When("storage class name is specified in both as parameters and in RabbitmqResourceBuilder instance", func() {
			BeforeEach(func() {
				instance.Spec.Persistence.StorageClassName = "my-storage-class"
				defaultConfiguration.PersistentStorageClassName = "a-storage-class-name"
			})
			It("creates the PersistentVolume template according to configurations in the RabbitmqResourceBuilder instance", func() {
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
				}
				statefulSet, _ := cluster.StatefulSet()
				Expect(*statefulSet.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(Equal("my-storage-class"))
			})
		})

		When("storage class name is specified only as parameters and not in RabbitmqResourceBuilder instance", func() {
			BeforeEach(func() {
				instance.Spec.Persistence.StorageClassName = ""
				defaultConfiguration.PersistentStorageClassName = "a-storage-class-name"
			})
			It("creates the PersistentVolume template according to the parameters", func() {
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
				}
				statefulSet, _ := cluster.StatefulSet()
				Expect(*statefulSet.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(Equal("a-storage-class-name"))
			})
		})

		When("storage class name is empty in parameters and is empty in RabbitmqResourceBuilder instance", func() {
			BeforeEach(func() {
				instance.Spec.Persistence.StorageClassName = ""
				defaultConfiguration.PersistentStorageClassName = ""
			})
			It("creates the PersistentVolume template with empty class so it defaults to  default StorageClass", func() {
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
				}
				statefulSet, _ := cluster.StatefulSet()
				Expect(statefulSet.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(BeNil())
			})
		})

		When("storage class capacity is specified in both as parameters and in RabbitmqResourceBuilder instance", func() {
			BeforeEach(func() {
				instance.Spec.Persistence.Storage = "21Gi"
				defaultConfiguration.PersistentStorage = "100Gi"
			})
			It("creates the PersistentVolume template according to configurations in the RabbitmqResourceBuilder instance", func() {
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
				}
				statefulSet, _ := cluster.StatefulSet()
				q, _ := k8sresource.ParseQuantity("21Gi")
				Expect(statefulSet.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests["storage"]).To(Equal(q))
			})
		})

		When("storage capacity is specified only as parameters and not in RabbitmqResourceBuilder instance", func() {
			BeforeEach(func() {
				instance.Spec.Persistence.Storage = ""
				defaultConfiguration.PersistentStorage = "100Gi"
			})
			It("creates the PersistentVolume template according to the parameters", func() {
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
				}
				statefulSet, _ := cluster.StatefulSet()
				q, _ := k8sresource.ParseQuantity("100Gi")
				Expect(statefulSet.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests["storage"]).To(Equal(q))
			})
		})

		When("storage capacity is empty in parameters and is empty in RabbitmqResourceBuilder instance", func() {
			BeforeEach(func() {
				instance.Spec.Persistence.StorageClassName = ""
				defaultConfiguration.PersistentStorage = ""
			})
			It("creates the PersistentVolume template with default capacity", func() {
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
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
				defaultConfiguration.ImagePullSecret = "ignored-operator-secret"

				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
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
					defaultConfiguration.ResourceRequirements = resource.ResourceRequirements{
						Limit: resource.ComputeResource{
							CPU:    "3m",
							Memory: "10Gi",
						},
						Request: resource.ComputeResource{
							CPU:    "",
							Memory: "",
						},
					}
				})

				Context("by the config", func() {
					It("generates a statefulSet with provided CPU and memory limits, with default CPU and memory requests", func() {
						cluster = &resource.RabbitmqResourceBuilder{
							Instance:             &instance,
							DefaultConfiguration: defaultConfiguration,
						}
						statefulSet, _ := cluster.StatefulSet()
						expectedCPULimit, _ := k8sresource.ParseQuantity("3m")
						expectedMemoryLimit, _ := k8sresource.ParseQuantity("10Gi")

						container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
						Expect(container.Resources.Limits[corev1.ResourceCPU]).To(Equal(expectedCPULimit))
						Expect(container.Resources.Requests[corev1.ResourceCPU]).To(Equal(k8sresource.MustParse(defaultCPURequest)))

						Expect(container.Resources.Limits[corev1.ResourceMemory]).To(Equal(expectedMemoryLimit))
						Expect(container.Resources.Requests[corev1.ResourceMemory]).To(Equal(k8sresource.MustParse(defaultMemoryRequest)))
					})
				})

				Context("both default resource requirements and CR resource requirements have been provided", func() {
					BeforeEach(func() {
						instance.Spec.Resource.Request = rabbitmqv1beta1.RabbitmqClusterComputeResource{
							CPU:    "10m",
							Memory: "3Gi",
						}

						instance.Spec.Resource.Limit = rabbitmqv1beta1.RabbitmqClusterComputeResource{
							CPU:    "11m",
							Memory: "4Gi",
						}

						defaultConfiguration.ResourceRequirements = resource.ResourceRequirements{
							Request: resource.ComputeResource{
								CPU:    "1m",
								Memory: "1Mi",
							},
							Limit: resource.ComputeResource{
								CPU:    "2m",
								Memory: "2Mi",
							},
						}
					})

					It("overrides StatefulSet resource requirements with those provided by CR spec", func() {
						cluster = &resource.RabbitmqResourceBuilder{
							Instance:             &instance,
							DefaultConfiguration: defaultConfiguration,
						}

						statefulSet, _ := cluster.StatefulSet()
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
				})
			})

			Context("CPU and memory requests are provided", func() {
				BeforeEach(func() {
					defaultConfiguration.ResourceRequirements = resource.ResourceRequirements{
						Limit: resource.ComputeResource{
							CPU:    "",
							Memory: "",
						},
						Request: resource.ComputeResource{
							CPU:    "10m",
							Memory: "5Gi",
						},
					}
				})

				It("generates a statefulSet with default CPU and memory limits", func() {
					cluster = &resource.RabbitmqResourceBuilder{
						Instance:             &instance,
						DefaultConfiguration: defaultConfiguration,
					}
					statefulSet, _ := cluster.StatefulSet()
					expectedCPURequest, _ := k8sresource.ParseQuantity("10m")
					expectedMemoryRequest, _ := k8sresource.ParseQuantity("5Gi")

					container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
					Expect(container.Resources.Limits[corev1.ResourceMemory]).To(Equal(k8sresource.MustParse(defaultMemoryLimit)))
					Expect(container.Resources.Requests[corev1.ResourceMemory]).To(Equal(expectedMemoryRequest))

					Expect(container.Resources.Limits[corev1.ResourceCPU]).To(Equal(k8sresource.MustParse(defaultCPULimit)))
					Expect(container.Resources.Requests[corev1.ResourceCPU]).To(Equal(expectedCPURequest))
				})
			})

			Context("both CPU and memory's limits plus requests are provided", func() {
				BeforeEach(func() {
					defaultConfiguration.ResourceRequirements = resource.ResourceRequirements{
						Limit: resource.ComputeResource{
							CPU:    "10m",
							Memory: "5Gi",
						},
						Request: resource.ComputeResource{
							CPU:    "1m",
							Memory: "1Gi",
						},
					}
				})

				It("generates a statefulSet with provided memory limit/request and provided CPU limit/request", func() {
					cluster = &resource.RabbitmqResourceBuilder{
						Instance:             &instance,
						DefaultConfiguration: defaultConfiguration,
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

		When("image repository and ImagePullSecret are provided through DefaultConfigurations", func() {
			It("uses the provided repository and references the instance registry secret name", func() {
				defaultConfiguration.ImageReference = "best-repository/rabbitmq:some-tag"
				defaultConfiguration.ImagePullSecret = "my-secret"
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
				}
				statefulSet, _ := cluster.StatefulSet()
				container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(container.Image).To(Equal("best-repository/rabbitmq:some-tag"))
				Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "foo-registry-access"}))
			})

			It("uses the instance ImagePullSecret and image reference if it is provided", func() {
				instance.Spec.Image = "my-private-repo/rabbitmq:latest"
				instance.Spec.ImagePullSecret = "my-great-secret"
				defaultConfiguration.ImageReference = "best-repository/rabbitmq:some-tag"
				defaultConfiguration.ImagePullSecret = "my-secret"
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
				}

				statefulSet, _ := cluster.StatefulSet()
				container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(container.Image).To(Equal("my-private-repo/rabbitmq:latest"))
				Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "my-great-secret"}))
			})
		})

		When("replica count is specified in the RabbitmqResourceBuilder spec", func() {
			It("sets the replica count of the StatefulSet to the provided value", func() {
				instance.Spec.Replicas = 3
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
				}
				statefulSet, _ := cluster.StatefulSet()
				Expect(*statefulSet.Spec.Replicas).To(Equal(int32(3)))
			})
		})
	})

	Context("label inheritance", func() {
		BeforeEach(func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{}
			instance.Namespace = "foo"
			instance.Name = "foo"
			instance.Labels = map[string]string{
				"app.kubernetes.io/foo": "bar",
				"foo":                   "bar",
				"rabbitmq":              "is-great",
				"foo/app.kubernetes.io": "edgecase",
			}

			scheme = runtime.NewScheme()
			rabbitmqv1beta1.AddToScheme(scheme)
			defaultscheme.AddToScheme(scheme)

			defaultConfiguration = resource.DefaultConfiguration{
				Scheme: scheme,
			}
			cluster = &resource.RabbitmqResourceBuilder{
				Instance:             &instance,
				DefaultConfiguration: defaultConfiguration,
			}
		})

		It("has the labels from the CRD on the statefulset", func() {
			statefulSet, err := cluster.StatefulSet()
			Expect(err).NotTo(HaveOccurred())
			testLabels(statefulSet.Labels)
		})

		It("has the labels from the CRD on the pod", func() {
			statefulSet, _ := cluster.StatefulSet()
			podTemplate := statefulSet.Spec.Template
			testLabels(podTemplate.Labels)
		})
	})

	Context("UpdateServiceParams", func() {
		var statefulSet *appsv1.StatefulSet

		BeforeEach(func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{}
			instance.Namespace = "foo"
			instance.Name = "foo"
			instance.Labels = map[string]string{
				"app.kubernetes.io/foo": "bar",
				"foo":                   "bar",
				"rabbitmq":              "is-great",
				"foo/app.kubernetes.io": "edgecase",
			}

			scheme = runtime.NewScheme()
			rabbitmqv1beta1.AddToScheme(scheme)
			defaultscheme.AddToScheme(scheme)

			defaultConfiguration = resource.DefaultConfiguration{
				Scheme: scheme,
			}
			cluster = &resource.RabbitmqResourceBuilder{
				Instance:             &instance,
				DefaultConfiguration: defaultConfiguration,
			}
			statefulSet = &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
						corev1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{Name: "pvc"},
						},
					},
				},
			}
		})

		It("adds labels from the CRD on the statefulset", func() {
			err := cluster.UpdateStatefulSetParams(statefulSet)
			Expect(err).NotTo(HaveOccurred())

			testLabels(statefulSet.Labels)
		})

		It("adds labels from the CRD on the pod", func() {
			err := cluster.UpdateStatefulSetParams(statefulSet)
			Expect(err).NotTo(HaveOccurred())

			podTemplate := statefulSet.Spec.Template
			testLabels(podTemplate.Labels)
		})

		It("sets nothing if the instance has no labels", func() {
			cluster.Instance.Labels = nil
			err := cluster.UpdateStatefulSetParams(statefulSet)
			Expect(err).NotTo(HaveOccurred())

			Expect(statefulSet.Labels).To(BeNil())
			pvcTemplate := statefulSet.Spec.VolumeClaimTemplates[0]
			Expect(pvcTemplate.Labels).To(BeNil())
			podTemplate := statefulSet.Spec.Template
			Expect(podTemplate.Labels).To(BeNil())
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
