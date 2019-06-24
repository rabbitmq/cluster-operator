package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("StatefulSet", func() {

	var instance rabbitmqv1beta1.RabbitmqCluster
	var sts *appsv1.StatefulSet

	BeforeEach(func() {

		instance = rabbitmqv1beta1.RabbitmqCluster{}
		instance.Namespace = "foo"
		instance.Name = "foo"
		sts = resource.GenerateStatefulSet(instance)
	})

	Context("Creates a working StatefulSet with minimum requirements", func() {

		It("with required Container Ports", func() {

			requiredContainerPorts := []int32{5672, 15672}
			var actualContainerPorts []int32

			container := extractContainer(sts, "rabbitmq")
			for _, port := range container.Ports {
				actualContainerPorts = append(actualContainerPorts, port.ContainerPort)
			}

			Expect(actualContainerPorts).Should(ConsistOf(requiredContainerPorts))
		})

		It("with required plugins and secrets Environment Variables", func() {

			requiredEnvVariables := []corev1.EnvVar{

				{
					Name:  "RABBITMQ_ENABLED_PLUGINS_FILE",
					Value: "/opt/rabbitmq-configmap/enabled_plugins",
				},
				{
					Name:  "RABBITMQ_DEFAULT_PASS_FILE",
					Value: "/opt/rabbitmq-secret/rabbitmq-password",
				},
				{
					Name:  "RABBITMQ_DEFAULT_USER_FILE",
					Value: "/opt/rabbitmq-secret/rabbitmq-username",
				},
			}

			container := extractContainer(sts, "rabbitmq")
			Expect(container.Env).Should(ConsistOf(requiredEnvVariables))
		})

		It("with required Config Map and Secret Volume Mounts", func() {

			configMapVolumeMount := corev1.VolumeMount{
				Name:      "rabbitmq-default-plugins",
				MountPath: "/opt/rabbitmq-configmap/",
			}
			secretVolumeMount := corev1.VolumeMount{
				Name:      "rabbitmq-secret",
				MountPath: "/opt/rabbitmq-secret/",
			}

			container := extractContainer(sts, "rabbitmq")
			Expect(container.VolumeMounts).Should(ConsistOf(configMapVolumeMount, secretVolumeMount))
		})

		It("with required Config Map and Secret Volumes", func() {
			configMapVolume := corev1.Volume{
				Name: "rabbitmq-default-plugins",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "rabbitmq-default-plugins",
						},
					},
				},
			}

			secretVolume := corev1.Volume{
				Name: "rabbitmq-secret",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "rabbitmq-secret",
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
			}

			Expect(sts.Spec.Template.Spec.Volumes).Should(ConsistOf(configMapVolume, secretVolume))
		})

	})

	Context("Creates a strongly recommended StatefulSet", func() {
		It("with Readiness Probe", func() {

			container := extractContainer(sts, "rabbitmq")
			actualProbeCommand := container.ReadinessProbe.Handler.Exec.Command
			Expect(actualProbeCommand).To(Equal([]string{"rabbitmq-diagnostics", "check_running"}))
		})
	})
})

func extractContainer(sts *appsv1.StatefulSet, containerName string) *corev1.Container {
	for _, container := range sts.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			return &container
		}
	}

	return &corev1.Container{}
}
