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

		It("with required Environment Variable", func() {

			requiredEnvVariables := []corev1.EnvVar{

				{
					Name:  "RABBITMQ_ENABLED_PLUGINS_FILE",
					Value: "/opt/rabbitmq-configmap/enabled_plugins",
				},
			}

			container := extractContainer(sts, "rabbitmq")
			Expect(container.Env).Should(ConsistOf(requiredEnvVariables))
		})

		It("with required Volume Mounts", func() {

			requiredVolumeMount := corev1.VolumeMount{
				Name:      "rabbitmq-default-plugins",
				MountPath: "/opt/rabbitmq-configmap/",
			}

			container := extractContainer(sts, "rabbitmq")
			Expect(container.VolumeMounts).Should(ConsistOf(requiredVolumeMount))
		})

		It("with required Volume", func() {

			requiredVolume := corev1.Volume{
				Name: "rabbitmq-default-plugins",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "rabbitmq-default-plugins",
						},
					},
				},
			}

			Expect(sts.Spec.Template.Spec.Volumes).Should(ConsistOf(requiredVolume))
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
