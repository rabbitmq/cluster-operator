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

const rabbitmqImageName = "rabbitmq:3.8.0-rc.1"

var _ = Describe("StatefulSet", func() {
	var (
		instance rabbitmqv1beta1.RabbitmqCluster
		scheme   *runtime.Scheme
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

			sts, _ = resource.GenerateStatefulSet(instance, "", "", "", "", scheme)
		})

		It("sets the right service name", func() {
			Expect(sts.Spec.ServiceName).To(Equal(instance.ChildResourceName("headless")))
		})

		It("sets replicas to be '1' by default", func() {
			Expect(*sts.Spec.Replicas).To(Equal(int32(1)))
		})

		It("adds the correct labels", func() {
			Expect(sts.Labels["app"]).To(Equal("pivotal-rabbitmq"))
			Expect(sts.Labels["RabbitmqCluster"]).To(Equal(instance.Name))
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
			secretName := instance.ChildResourceName("server")
			requiredEnvVariables := []corev1.EnvVar{
				{
					Name: "RABBITMQ_ERLANG_COOKIE",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secretName,
							},
							Key: "cookie",
						},
					},
				},
				{
					Name: "RABBITMQ_DEFAULT_PASS",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secretName,
							},
							Key: "password",
						},
					},
				},
				{
					Name: "RABBITMQ_DEFAULT_USER",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secretName,
							},
							Key: "username",
						},
					},
				},
				{
					Name:  "RABBITMQ_ENABLED_PLUGINS_FILE",
					Value: "/opt/server-conf/enabled_plugins",
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
					Name:      "persistence",
					MountPath: "/var/lib/rabbitmq/db/",
				},
				corev1.VolumeMount{
					Name:      "rabbitmq-etc",
					MountPath: "/etc/rabbitmq/",
				},
			))
		})

		It("defines the expected volumes", func() {
			Expect(sts.Spec.Template.Spec.Volumes).Should(ConsistOf(
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
						"app": "foo",
					},
					OwnerReferences: []v1.OwnerReference{
						{
							APIVersion:         "rabbitmq.pivotal.io/v1beta1",
							Kind:               "RabbitmqCluster",
							Name:               "foo",
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
				FSGroup:   &rmqGID,
				RunAsUser: &rmqUID,
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
			Expect(container.Image).To(Equal(rabbitmqImageName))
			Expect(sts.Spec.Template.Spec.ImagePullSecrets).To(BeEmpty())
		})

		It("templates the correct InitContainer", func() {
			initContainers := sts.Spec.Template.Spec.InitContainers
			Expect(len(initContainers)).To(Equal(1))

			container := extractContainer(initContainers, "copy-config")
			Expect(container.Command).To(Equal([]string{
				"sh", "-c", "cp /tmp/rabbitmq/rabbitmq.conf /etc/rabbitmq/rabbitmq.conf && echo '' >> /etc/rabbitmq/rabbitmq.conf",
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
			))

			Expect(container.Image).To(Equal(rabbitmqImageName))
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
			It("creates the PersistentVolume template according to configurations in the RabbitmqCluster instance", func() {
				instance.Spec.Persistence.StorageClassName = "my-storage-class"

				statefulSet, _ := resource.GenerateStatefulSet(instance, "", "", "some-other-storage-name", "", scheme)
				Expect(*statefulSet.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(Equal("my-storage-class"))
			})
		})

		When("storage class name is specified only as parameters and not in RabbitmqCluster instance", func() {
			It("creates the PersistentVolume template according to the parameters", func() {
				instance.Spec.Persistence.StorageClassName = ""

				statefulSet, _ := resource.GenerateStatefulSet(instance, "", "", "a-storage-class-name", "100Gi", scheme)
				Expect(*statefulSet.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(Equal("a-storage-class-name"))
			})
		})

		When("storage class name is empty in parameters and is empty in RabbitmqCluster instance", func() {
			It("creates the PersistentVolume template with empty class so it defaults to  default StorageClass", func() {
				instance.Spec.Persistence.StorageClassName = ""

				statefulSet, _ := resource.GenerateStatefulSet(instance, "", "", "", "", scheme)
				Expect(statefulSet.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(BeNil())
			})
		})

		When("storage class capacity is specified in both as parameters and in RabbitmqCluster instance", func() {
			It("creates the PersistentVolume template according to configurations in the RabbitmqCluster instance", func() {
				instance.Spec.Persistence.Storage = "21Gi"

				statefulSet, _ := resource.GenerateStatefulSet(instance, "", "", "", "41Gi", scheme)
				q, _ := k8sresource.ParseQuantity("21Gi")
				Expect(statefulSet.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests["storage"]).To(Equal(q))
			})
		})

		When("storage capacity is specified only as parameters and not in RabbitmqCluster instance", func() {
			It("creates the PersistentVolume template according to the parameters", func() {
				instance.Spec.Persistence.Storage = ""

				statefulSet, _ := resource.GenerateStatefulSet(instance, "", "", "", "100Gi", scheme)
				q, _ := k8sresource.ParseQuantity("100Gi")
				Expect(statefulSet.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests["storage"]).To(Equal(q))
			})
		})

		When("storage capacity is empty in parameters and is empty in RabbitmqCluster instance", func() {
			It("creates the PersistentVolume template with default capacity", func() {
				instance.Spec.Persistence.StorageClassName = ""

				statefulSet, _ := resource.GenerateStatefulSet(instance, "", "", "", "", scheme)
				q, _ := k8sresource.ParseQuantity("10Gi")
				Expect(statefulSet.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests["storage"]).To(Equal(q))
			})
		})

		When("configuring a private image repository", func() {
			It("templates the image string and the imagePullSecrets correctly", func() {
				instance.Spec.Image.Repository = "my-private-repo"
				instance.Spec.ImagePullSecret = "my-great-secret"

				statefulSet, _ := resource.GenerateStatefulSet(instance, "", "", "", "", scheme)
				container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(container.Image).To(Equal("my-private-repo/" + rabbitmqImageName))
				Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "my-great-secret"}))

				initContainer := extractContainer(statefulSet.Spec.Template.Spec.InitContainers, "copy-config")
				Expect(initContainer.Image).To(Equal("my-private-repo/" + rabbitmqImageName))
			})
		})

		When("image repository and ImagePullSecret are provided through function params", func() {
			It("uses the provide repository and secret if not specified in RabbitmqCluster spec", func() {
				statefulSet, _ := resource.GenerateStatefulSet(instance, "best-repository", "my-secret", "", "", scheme)
				container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(container.Image).To(Equal("best-repository/" + rabbitmqImageName))
				Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "my-secret"}))
			})

			It("uses the RabbitmqCluster spec if it is provided", func() {
				instance.Spec.Image.Repository = "my-private-repo"
				instance.Spec.ImagePullSecret = "my-great-secret"
				statefulSet, _ := resource.GenerateStatefulSet(instance, "best-repository", "my-secret", "", "", scheme)
				container := extractContainer(statefulSet.Spec.Template.Spec.Containers, "rabbitmq")
				Expect(container.Image).To(Equal("my-private-repo/" + rabbitmqImageName))
				Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "my-great-secret"}))
			})
		})

		When("replica count is specified in the RabbitmqCluster spec", func() {
			It("sets the replica count of the StatefulSet to the provided value", func() {
				instance.Spec.Replicas = 3
				statefulSet, _ := resource.GenerateStatefulSet(instance, "", "", "", "", scheme)
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
