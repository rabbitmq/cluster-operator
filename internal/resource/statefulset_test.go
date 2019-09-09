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
	var instance rabbitmqv1beta1.RabbitmqCluster
	var sts *appsv1.StatefulSet
	var secretName, configMapName string
	var scheme *runtime.Scheme

	BeforeEach(func() {
		instance = rabbitmqv1beta1.RabbitmqCluster{}
		instance.Namespace = "foo"
		instance.Name = "foo"
		secretName = instance.Name + "-rabbitmq-admin"
		configMapName = instance.Name + "-rabbitmq-plugins"
		scheme = runtime.NewScheme()
		rabbitmqv1beta1.AddToScheme(scheme)
		defaultscheme.AddToScheme(scheme)
		sts, _ = resource.GenerateStatefulSet(instance, "", "", "", "", scheme)
	})

	Context("when creating a working StatefulSet with minimum requirements", func() {

		It("adds the correct labels", func() {
			Expect(sts.Labels["app"]).To(Equal("pivotal-rabbitmq"))
			Expect(sts.Labels["RabbitmqCluster"]).To(Equal(instance.Name))
		})

		It("adds the correct name with naming conventions", func() {
			expectedName := instance.Name + StatefulSetSuffix
			Expect(sts.Name).To(Equal(expectedName))
		})

		It("specifies required Container Ports", func() {

			requiredContainerPorts := []int32{5672, 15672, 15692}
			var actualContainerPorts []int32

			container := extractContainer(sts, "rabbitmq")
			for _, port := range container.Ports {
				actualContainerPorts = append(actualContainerPorts, port.ContainerPort)
			}

			Expect(actualContainerPorts).Should(ConsistOf(requiredContainerPorts))
		})

		It("uses required Environment Variables", func() {

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
				{
					Name:  "RABBITMQ_MNESIA_BASE",
					Value: "/opt/rabbitmq-persistence",
				},
			}

			container := extractContainer(sts, "rabbitmq")
			Expect(container.Env).Should(ConsistOf(requiredEnvVariables))
		})

		It("creates required Volume Mounts", func() {

			configMapVolumeMount := corev1.VolumeMount{
				Name:      "rabbitmq-plugins",
				MountPath: "/opt/rabbitmq-configmap/",
			}
			secretVolumeMount := corev1.VolumeMount{
				Name:      "rabbitmq-admin",
				MountPath: "/opt/rabbitmq-secret/",
			}
			persistenceVolumeMount := corev1.VolumeMount{
				Name:      "persistence",
				MountPath: "/opt/rabbitmq-persistence/",
			}

			container := extractContainer(sts, "rabbitmq")
			Expect(container.VolumeMounts).Should(ConsistOf(configMapVolumeMount, secretVolumeMount, persistenceVolumeMount))
		})

		It("uses required Config Map and Secret Volumes", func() {
			configMapVolume := corev1.Volume{
				Name: "rabbitmq-plugins",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: configMapName,
						},
					},
				},
			}

			secretVolume := corev1.Volume{
				Name: "rabbitmq-admin",
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

		It("creates the required PersistentVolumeClaim", func() {
			truth := true
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
							corev1.ResourceStorage: *k8sresource.NewQuantity(10*1024*1024*1024, k8sresource.BinarySI),
						},
					},
				},
			}

			actualPersistentVolumeClaim := sts.Spec.VolumeClaimTemplates[0]
			Expect(actualPersistentVolumeClaim).To(Equal(expectedPersistentVolumeClaim))
		})

		It("creates the required SecurityContext", func() {
			rmqGid := int64(999)
			expectedPodSecurityContext := &corev1.PodSecurityContext{
				FSGroup: &rmqGid,
			}

			actualPodSecurityContext := sts.Spec.Template.Spec.SecurityContext
			Expect(actualPodSecurityContext).To(Equal(expectedPodSecurityContext))
		})
	})

	Context("when creating a strongly recommended StatefulSet", func() {
		It("defines a Readiness Probe", func() {

			container := extractContainer(sts, "rabbitmq")
			actualProbeCommand := container.ReadinessProbe.Handler.Exec.Command
			Expect(actualProbeCommand).To(Equal([]string{"rabbitmq-diagnostics", "check_running"}))
		})
	})

	Context("storage class name and capacity", func() {
		When("storage class name and capacity is specified in both as parameters and in RabbitmqCluster instance", func() {
			It("creates the PersistentVolume template according to configurations in the RabbitmqCluster instance", func() {
				instance.Spec.Persistence.StorageClassName = "my-storage-class"
				instance.Spec.Persistence.Storage = "1Gi"

				statefulSet, _ := resource.GenerateStatefulSet(instance, "", "", "", "", scheme)
				q, _ := k8sresource.ParseQuantity("1Gi")
				Expect(statefulSet.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests["storage"]).To(Equal(q))
				Expect(*statefulSet.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(Equal("my-storage-class"))
			})
		})

		When("storage class name and capacity is specified only as parameters and not in RabbitmqCluster instance", func() {
			It("creates the PersistentVolume template according to the parameters", func() {
				statefulSet, _ := resource.GenerateStatefulSet(instance, "", "", "a-storage-class-name", "100Gi", scheme)
				q, _ := k8sresource.ParseQuantity("100Gi")
				Expect(statefulSet.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests["storage"]).To(Equal(q))
				Expect(*statefulSet.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(Equal("a-storage-class-name"))
			})
		})
	})

	Context("Image and ImagePullSecrets", func() {

		Context("when configuring a private image repository", func() {
			It("templates the image string and the imagePullSecrets correctly", func() {
				instance.Spec.Image.Repository = "my-private-repo"
				instance.Spec.ImagePullSecret = "my-great-secret"

				statefulSet, _ := resource.GenerateStatefulSet(instance, "", "", "", "", scheme)
				container := extractContainer(statefulSet, "rabbitmq")
				Expect(container.Image).To(Equal("my-private-repo/" + "rabbitmq:3.8-rc-management"))
				Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "my-great-secret"}))
			})
		})

		Context("when not configuring a private image repository", func() {
			It("templates the image string and the imagePullSecrets with default values", func() {
				container := extractContainer(sts, "rabbitmq")
				Expect(container.Image).To(Equal(resource.RabbitmqManagementImage))
				Expect(sts.Spec.Template.Spec.ImagePullSecrets).To(BeEmpty())
			})
		})
	})

	Context("when image repository and ImagePullSecret is provided through function params", func() {
		It("uses the provide repository and secret if not specified in RabbitmqCluster spec", func() {
			statefulSet, _ := resource.GenerateStatefulSet(instance, "best-repository", "my-secret", "", "", scheme)
			container := extractContainer(statefulSet, "rabbitmq")
			Expect(container.Image).To(Equal("best-repository/" + "rabbitmq:3.8-rc-management"))
			Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "my-secret"}))
		})

		It("uses the RabbitmqCluster spec if it is provided", func() {
			instance.Spec.Image.Repository = "my-private-repo"
			instance.Spec.ImagePullSecret = "my-great-secret"
			statefulSet, _ := resource.GenerateStatefulSet(instance, "best-repository", "my-secret", "", "", scheme)
			container := extractContainer(statefulSet, "rabbitmq")
			Expect(container.Image).To(Equal("my-private-repo/" + "rabbitmq:3.8-rc-management"))
			Expect(statefulSet.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "my-great-secret"}))
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
