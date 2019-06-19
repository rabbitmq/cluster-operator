package resource

import (
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenerateStatefulSet(instance rabbitmqv1beta1.RabbitmqCluster) *appsv1.StatefulSet {
	single := int32(1)

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
					Containers: []corev1.Container{
						{
							Name:  "rabbitmq",
							Image: "rabbitmq:3.8-rc-management",
							Env: []corev1.EnvVar{
								{
									Name:  "RABBITMQ_ENABLED_PLUGINS_FILE",
									Value: "/opt/rabbitmq-configmap/enabled_plugins",
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
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "rabbitmq-default-plugins",
									MountPath: "/opt/rabbitmq-configmap/",
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
										Name: "rabbitmq-default-plugins",
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
