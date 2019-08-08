package resource

import (
	"fmt"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const RabbitmqManagementImage string = "rabbitmq:3.8-rc-management"

func GenerateStatefulSet(instance rabbitmqv1beta1.RabbitmqCluster, imageRepository, imagePullSecret string) *appsv1.StatefulSet {
	single := int32(1)
	f := false
	image := RabbitmqManagementImage

	if instance.Spec.Image.Repository != "" {
		image = fmt.Sprintf("%s/%s", instance.Spec.Image.Repository, image)
	} else if imageRepository != "" {
		image = fmt.Sprintf("%s/%s", imageRepository, image)
	}

	imagePullSecrets := []corev1.LocalObjectReference{}
	if instance.Spec.ImagePullSecret != "" {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: instance.Spec.ImagePullSecret})
	} else if imagePullSecret != "" {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: imagePullSecret})
	}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "p-" + instance.Name,
			Namespace: instance.Namespace,
			Labels: map[string]string{
				"app": instance.Name,
			},
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: instance.Name,
			Replicas:    &single,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": instance.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": instance.Name}},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: &f,
					ImagePullSecrets:             imagePullSecrets,
					Containers: []corev1.Container{
						{
							Name:  "rabbitmq",
							Image: image,
							Env: []corev1.EnvVar{
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
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "amqp",
									ContainerPort: 5672,
								},
								{
									Name:          "http",
									ContainerPort: 15672,
								},
								{
									Name:          "prometheus",
									ContainerPort: 15692,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "rabbitmq-default-plugins",
									MountPath: "/opt/rabbitmq-configmap/",
								},
								{
									Name:      "rabbitmq-secret",
									MountPath: "/opt/rabbitmq-secret/",
								},
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{"rabbitmq-diagnostics", "check_running"},
									},
								},
								InitialDelaySeconds: 10,
								TimeoutSeconds:      5,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								FailureThreshold:    3,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "rabbitmq-default-plugins",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: instance.Name + "-rabbitmq-default-plugins",
									},
								},
							},
						},
						{
							Name: "rabbitmq-secret",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: instance.Name + "-rabbitmq-secret",
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
					},
				},
			},
		},
	}
}
