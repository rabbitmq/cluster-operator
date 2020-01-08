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
		sts                  *appsv1.StatefulSet
	)

	Context("Build with default settings", func() {
		BeforeEach(func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: v1.ObjectMeta{
					Name:      "foo",
					Namespace: "foo",
				},
			}
			scheme = runtime.NewScheme()
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
			Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())

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
			stsBuilder := cluster.StatefulSet()
			obj, _ := stsBuilder.Build()
			sts = obj.(*appsv1.StatefulSet)
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

		It("creates the PersistentVolume template with empty class so it defaults to default StorageClass", func() {
			Expect(sts.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(BeNil())
		})

		It("creates the PersistentVolume template with default capacity", func() {
			q, _ := k8sresource.ParseQuantity("10Gi")
			Expect(sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests["storage"]).To(Equal(q))
		})
	})

	Context("Build with non-default settings", func() {
		BeforeEach(func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: v1.ObjectMeta{
					Name:      "foo",
					Namespace: "foo",
				},
			}

			scheme = runtime.NewScheme()
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
			Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())

			defaultConfiguration = resource.DefaultConfiguration{
				Scheme: scheme,
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
			instance.Spec.Affinity = affinity
			cluster = &resource.RabbitmqResourceBuilder{
				Instance:             &instance,
				DefaultConfiguration: defaultConfiguration,
			}
			stsBuilder := cluster.StatefulSet()
			obj, _ := stsBuilder.Build()
			sts = obj.(*appsv1.StatefulSet)
			Expect(sts.Spec.Template.Spec.Affinity).To(Equal(affinity))
		})

		Context("Tolerations", func() {
			It("creates the tolerations specified", func() {
				tolerations := []corev1.Toleration{
					{
						Key:      "key",
						Operator: "equals",
						Value:    "value",
						Effect:   "NoSchedule",
					},
				}

				instance.Spec.Tolerations = tolerations
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
				}
				stsBuilder := cluster.StatefulSet()
				obj, _ := stsBuilder.Build()
				sts = obj.(*appsv1.StatefulSet)
				Expect(sts.Spec.Template.Spec.Tolerations).To(Equal(tolerations))
			})
		})

		Context("Storage class name", func() {
			It("creates the PersistentVolume template according to configurations in the  instance if specified", func() {
				instance.Spec.Persistence.StorageClassName = "my-storage-class"
				defaultConfiguration.PersistentStorageClassName = "a-storage-class-name"
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
				}
				stsBuilder := cluster.StatefulSet()
				obj, _ := stsBuilder.Build()
				sts = obj.(*appsv1.StatefulSet)
				Expect(*sts.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(Equal("my-storage-class"))
			})

			It("creates the PersistentVolume template according to the parameters if not specified in the instance", func() {
				instance.Spec.Persistence.StorageClassName = ""
				defaultConfiguration.PersistentStorageClassName = "a-storage-class-name"
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
				}
				stsBuilder := cluster.StatefulSet()
				obj, _ := stsBuilder.Build()
				sts = obj.(*appsv1.StatefulSet)
				Expect(*sts.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(Equal("a-storage-class-name"))
			})
		})

		Context("Storage capacity", func() {

			It("creates the PersistentVolume template according to configurations in the  instance", func() {
				instance.Spec.Persistence.Storage = "21Gi"
				defaultConfiguration.PersistentStorage = "100Gi"
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
				}
				stsBuilder := cluster.StatefulSet()
				obj, _ := stsBuilder.Build()
				sts = obj.(*appsv1.StatefulSet)
				q, _ := k8sresource.ParseQuantity("21Gi")
				Expect(sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests["storage"]).To(Equal(q))
			})

			It("creates the PersistentVolume template according to the parameters if not specified in the instance", func() {
				instance.Spec.Persistence.Storage = ""
				defaultConfiguration.PersistentStorage = "100Gi"
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
				}
				stsBuilder := cluster.StatefulSet()
				obj, _ := stsBuilder.Build()
				sts = obj.(*appsv1.StatefulSet)
				q, _ := k8sresource.ParseQuantity("100Gi")
				Expect(sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests["storage"]).To(Equal(q))
			})
		})

		Context("resources requirements", func() {

			It("sets defaults if none are provided", func() {
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
				}
				stsBuilder := cluster.StatefulSet()
				obj, _ := stsBuilder.Build()
				sts = obj.(*appsv1.StatefulSet)

				container := extractContainer(sts.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(container.Resources.Limits[corev1.ResourceCPU]).To(Equal(k8sresource.MustParse(defaultCPULimit)))
				Expect(container.Resources.Requests[corev1.ResourceCPU]).To(Equal(k8sresource.MustParse(defaultCPURequest)))

				Expect(container.Resources.Limits[corev1.ResourceMemory]).To(Equal(k8sresource.MustParse(defaultMemoryLimit)))
				Expect(container.Resources.Requests[corev1.ResourceMemory]).To(Equal(k8sresource.MustParse(defaultMemoryRequest)))
			})

			It("sets requirements from DefaultConfiguration", func() {
				defaultConfiguration.ResourceRequirements = resource.ResourceRequirements{
					Limit: resource.ComputeResource{
						CPU:    "3m",
						Memory: "10Gi",
					},
					Request: resource.ComputeResource{
						CPU:    "30m",
						Memory: "1Gi",
					},
				}
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
				}
				stsBuilder := cluster.StatefulSet()
				obj, _ := stsBuilder.Build()
				sts = obj.(*appsv1.StatefulSet)
				expectedCPULimit, _ := k8sresource.ParseQuantity("3m")
				expectedCPURequest, _ := k8sresource.ParseQuantity("30m")
				expectedMemoryRequest, _ := k8sresource.ParseQuantity("1Gi")
				expectedMemoryLimit, _ := k8sresource.ParseQuantity("10Gi")

				container := extractContainer(sts.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(container.Resources.Limits[corev1.ResourceCPU]).To(Equal(expectedCPULimit))
				Expect(container.Resources.Requests[corev1.ResourceCPU]).To(Equal(expectedCPURequest))

				Expect(container.Resources.Limits[corev1.ResourceMemory]).To(Equal(expectedMemoryLimit))
				Expect(container.Resources.Requests[corev1.ResourceMemory]).To(Equal(expectedMemoryRequest))
			})

			It("combines values from operator defaults and DefaultConfiguration", func() {
				defaultConfiguration.ResourceRequirements = resource.ResourceRequirements{
					Limit: resource.ComputeResource{
						CPU:    "3m",
						Memory: "",
					},
					Request: resource.ComputeResource{
						CPU:    "",
						Memory: "1Gi",
					},
				}
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
				}
				stsBuilder := cluster.StatefulSet()
				obj, _ := stsBuilder.Build()
				sts = obj.(*appsv1.StatefulSet)
				expectedCPULimit, _ := k8sresource.ParseQuantity("3m")
				expectedMemoryRequest, _ := k8sresource.ParseQuantity("1Gi")

				container := extractContainer(sts.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(container.Resources.Limits[corev1.ResourceCPU]).To(Equal(expectedCPULimit))
				Expect(container.Resources.Requests[corev1.ResourceCPU]).To(Equal(k8sresource.MustParse(defaultCPURequest)))

				Expect(container.Resources.Limits[corev1.ResourceMemory]).To(Equal(k8sresource.MustParse(defaultMemoryRequest)))
				Expect(container.Resources.Requests[corev1.ResourceMemory]).To(Equal(expectedMemoryRequest))
			})

			It("overrides StatefulSet resource requirements with those provided by CR spec", func() {
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
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
				}

				stsBuilder := cluster.StatefulSet()
				obj, _ := stsBuilder.Build()
				sts = obj.(*appsv1.StatefulSet)
				expectedCPURequest, _ := k8sresource.ParseQuantity("10m")
				expectedMemoryRequest, _ := k8sresource.ParseQuantity("3Gi")
				expectedCPULimit, _ := k8sresource.ParseQuantity("11m")
				expectedMemoryLimit, _ := k8sresource.ParseQuantity("4Gi")

				container := extractContainer(sts.Spec.Template.Spec.Containers, "rabbitmq")
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

				defaultConfiguration.ResourceRequirements = resource.ResourceRequirements{
					Limit: resource.ComputeResource{
						CPU:    "2m",
						Memory: "2Mi",
					},
				}
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
				}

				stsBuilder := cluster.StatefulSet()
				obj, _ := stsBuilder.Build()
				sts = obj.(*appsv1.StatefulSet)

				container := extractContainer(sts.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(len(container.Resources.Requests)).To(Equal(0))
				Expect(len(container.Resources.Limits)).To(Equal(0))
			})
		})

		When("configures private image", func() {
			It("uses the provided repository and registry secret name from DefaultConfiguration", func() {
				defaultConfiguration.ImageReference = "best-repository/rabbitmq:some-tag"
				defaultConfiguration.ImagePullSecret = "my-secret"
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
				}
				stsBuilder := cluster.StatefulSet()
				obj, _ := stsBuilder.Build()
				sts = obj.(*appsv1.StatefulSet)
				container := extractContainer(sts.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(container.Image).To(Equal("best-repository/rabbitmq:some-tag"))
				Expect(sts.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "foo-registry-access"}))
			})

			It("uses the instance ImagePullSecret and image reference when provided", func() {
				instance.Spec.Image = "my-private-repo/rabbitmq:latest"
				instance.Spec.ImagePullSecret = "my-great-secret"
				defaultConfiguration.ImageReference = "best-repository/rabbitmq:some-tag"
				defaultConfiguration.ImagePullSecret = "my-secret"
				cluster = &resource.RabbitmqResourceBuilder{
					Instance:             &instance,
					DefaultConfiguration: defaultConfiguration,
				}

				stsBuilder := cluster.StatefulSet()
				obj, _ := stsBuilder.Build()
				sts = obj.(*appsv1.StatefulSet)
				container := extractContainer(sts.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(container.Image).To(Equal("my-private-repo/rabbitmq:latest"))
				Expect(sts.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "my-great-secret"}))
			})
		})

		It("sets the replica count of the StatefulSet to the instance value", func() {
			instance.Spec.Replicas = 3
			cluster = &resource.RabbitmqResourceBuilder{
				Instance:             &instance,
				DefaultConfiguration: defaultConfiguration,
			}
			stsBuilder := cluster.StatefulSet()
			obj, _ := stsBuilder.Build()
			sts = obj.(*appsv1.StatefulSet)
			Expect(*sts.Spec.Replicas).To(Equal(int32(3)))
		})
	})

	Context("Build with instance labels", func() {
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
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
			Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())

			defaultConfiguration = resource.DefaultConfiguration{
				Scheme: scheme,
			}
			cluster = &resource.RabbitmqResourceBuilder{
				Instance:             &instance,
				DefaultConfiguration: defaultConfiguration,
			}
		})

		It("has the labels from the instance on the statefulset", func() {
			stsBuilder := cluster.StatefulSet()
			obj, _ := stsBuilder.Build()
			sts = obj.(*appsv1.StatefulSet)
			testLabels(sts.Labels)
		})

		It("has the labels from the instance on the pod", func() {
			stsBuilder := cluster.StatefulSet()
			obj, _ := stsBuilder.Build()
			sts = obj.(*appsv1.StatefulSet)
			podTemplate := sts.Spec.Template
			testLabels(podTemplate.Labels)
		})
	})

	Context("Build with instance annotations", func() {
		BeforeEach(func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{}
			instance.Namespace = "foo"
			instance.Name = "foo"
			instance.Annotations = map[string]string{
				"my-annotation":              "i-like-this",
				"kubernetes.io/name":         "i-do-not-like-this",
				"kubectl.kubernetes.io/name": "i-do-not-like-this",
				"k8s.io/name":                "i-do-not-like-this",
			}

			scheme = runtime.NewScheme()
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
			Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())

			defaultConfiguration = resource.DefaultConfiguration{
				Scheme: scheme,
			}
			cluster = &resource.RabbitmqResourceBuilder{
				Instance:             &instance,
				DefaultConfiguration: defaultConfiguration,
			}
			stsBuilder := cluster.StatefulSet()
			obj, _ := stsBuilder.Build()
			sts = obj.(*appsv1.StatefulSet)
		})

		It("has the annotations from the instance on the StatefulSet", func() {
			testAnnotations(sts.Annotations)
		})

		It("has the annotations from the instance on the pod", func() {
			podTemplate := sts.Spec.Template
			testAnnotations(podTemplate.Annotations)
		})
	})

	Context("Update", func() {
		var (
			statefulSet          *appsv1.StatefulSet
			stsBuilder           *resource.StatefulSetBuilder
			existingLabels       map[string]string
			existingAnnotations  map[string]string
			affinity             *corev1.Affinity
			resourceRequirements corev1.ResourceRequirements
		)

		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
			Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())

			defaultConfiguration = resource.DefaultConfiguration{
				Scheme: scheme,
			}

			cluster = &resource.RabbitmqResourceBuilder{
				Instance:             &instance,
				DefaultConfiguration: defaultConfiguration,
			}
			existingLabels = map[string]string{
				"app.kubernetes.io/name":      instance.Name,
				"app.kubernetes.io/part-of":   "pivotal-rabbitmq",
				"this-was-the-previous-label": "should-be-deleted",
			}

			existingAnnotations = map[string]string{
				"this-was-the-previous-annotation": "should-be-deleted",
			}

			stsBuilder = cluster.StatefulSet()

			statefulSet = &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      existingLabels,
					Annotations: existingAnnotations,
				},
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: existingLabels,
						},
						Spec: corev1.PodSpec{
							InitContainers: []corev1.Container{{}},
							Containers:     []corev1.Container{{}},
						},
					},
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

		It("adds the resource requirements", func() {
			resourceRequirements = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: k8sresource.MustParse("300m"),
				},
			}
			stsBuilder.Instance.Spec.Resources = &resourceRequirements

			Expect(stsBuilder.Update(statefulSet)).To(Succeed())
			Expect(statefulSet.Spec.Template.Spec.Containers[0].Resources).To(Equal(resourceRequirements))
		})

		It("updates labels", func() {
			stsBuilder.Instance.Labels = map[string]string{
				"app.kubernetes.io/foo": "bar",
				"foo":                   "bar",
				"rabbitmq":              "is-great",
				"foo/app.kubernetes.io": "edgecase",
			}
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

		It("updates annotations", func() {
			stsBuilder.Instance.Annotations = map[string]string{
				"my-annotation":              "i-like-this",
				"kubernetes.io/name":         "i-do-not-like-this",
				"kubectl.kubernetes.io/name": "i-do-not-like-this",
				"k8s.io/name":                "i-do-not-like-this",
			}
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())

			testAnnotations(statefulSet.Annotations)
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

		It("updates the rabbitmq image and the init container image", func() {
			stsBuilder.Instance.Spec.Image = "rabbitmq:3.8.0"
			Expect(stsBuilder.Update(statefulSet)).To(Succeed())
			Expect(statefulSet.Spec.Template.Spec.Containers[0].Image).To(Equal("rabbitmq:3.8.0"))
			Expect(statefulSet.Spec.Template.Spec.InitContainers[0].Image).To(Equal("rabbitmq:3.8.0"))
		})

		Context("updates labels on pod", func() {
			BeforeEach(func() {
				statefulSet.Spec.Template.Labels = existingLabels
			})

			It("adds labels from the CR to the pod", func() {
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())

				testLabels(statefulSet.Spec.Template.Labels)
			})

			It("restores the default labels", func() {
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())

				labels := statefulSet.Spec.Template.Labels
				Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
				Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
				Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
			})

			It("deletes the labels that are removed from the CR", func() {
				Expect(stsBuilder.Update(statefulSet)).To(Succeed())

				Expect(statefulSet.Spec.Template.Labels).NotTo(HaveKey("this-was-the-previous-label"))
			})
		})

		Context("updates annotations on pod", func() {
			BeforeEach(func() {
				statefulSet.Spec.Template.Annotations = existingAnnotations
			})

			It("update labels from the instance to the pod", func() {
				stsBuilder.Instance.Annotations = map[string]string{
					"my-annotation":              "i-like-this",
					"kubernetes.io/name":         "i-do-not-like-this",
					"kubectl.kubernetes.io/name": "i-do-not-like-this",
					"k8s.io/name":                "i-do-not-like-this",
				}

				Expect(stsBuilder.Update(statefulSet)).To(Succeed())
				testAnnotations(statefulSet.Spec.Template.Annotations)
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
