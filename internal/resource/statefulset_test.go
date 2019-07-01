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
	var secretName, configMapName string

	BeforeEach(func() {
		instance = rabbitmqv1beta1.RabbitmqCluster{}
		instance.Namespace = "foo"
		instance.Name = "foo"
		secretName = instance.Name + "-rabbitmq-secret"
		configMapName = instance.Name + "-rabbitmq-default-plugins"
		sts = resource.GenerateStatefulSet(instance)
	})

	Context("when creating a working StatefulSet with minimum requirements", func() {

		It("specifies required Container Ports", func() {

			requiredContainerPorts := []int32{5672, 15672}
			var actualContainerPorts []int32

			container := extractContainer(sts, "rabbitmq")
			for _, port := range container.Ports {
				actualContainerPorts = append(actualContainerPorts, port.ContainerPort)
			}

			Expect(actualContainerPorts).Should(ConsistOf(requiredContainerPorts))
		})

		It("uses required plugins and secrets Environment Variables", func() {

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

		It("creates required Config Map and Secret Volume Mounts", func() {

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

		It("uses required Config Map and Secret Volumes", func() {
			configMapVolume := corev1.Volume{
				Name: "rabbitmq-default-plugins",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: configMapName,
						},
					},
				},
			}

			secretVolume := corev1.Volume{
				Name: "rabbitmq-secret",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secretName,
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

		It("does not mount the default service account in its pods", func() {
			Expect(*sts.Spec.Template.Spec.AutomountServiceAccountToken).To(BeFalse())
		})
	})

	Context("when creating a strongly recommended StatefulSet", func() {
		It("defines a Readiness Probe", func() {

			container := extractContainer(sts, "rabbitmq")
			actualProbeCommand := container.ReadinessProbe.Handler.Exec.Command
			Expect(actualProbeCommand).To(Equal([]string{"rabbitmq-diagnostics", "check_running"}))
		})
	})

	Context("Image and ImagePullSecrets", func() {
		Context("when configuring a private image repository", func() {
			It("templates the image string and the imagePullSecrets correctly", func() {
				instance.Spec.Image.Repository = "my-private-repo"
				instance.Spec.ImagePullSecret = "my-great-secret"

				statefulSet := resource.GenerateStatefulSet(instance)
				container := extractContainer(statefulSet, "rabbitmq")
				Expect(container.Image).To(Equal("my-private-repo/rabbitmq:3.8-rc-management"))
				Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "my-great-secret"}))
			})
		})

		Context("when not configuring a private image repository", func() {
			It("templates the image string and the imagePullSecrets with default values", func() {
				container := extractContainer(sts, "rabbitmq")
				Expect(container.Image).To(Equal("rabbitmq:3.8-rc-management"))
				Expect(sts.Spec.Template.Spec.ImagePullSecrets).To(BeEmpty())
			})
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
