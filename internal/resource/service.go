package resource

import (
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenerateHeadlessService(instance rabbitmqv1beta1.RabbitmqCluster) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.ChildResourceName("rabbitmq-headless"),
			Namespace: instance.Namespace,
			Labels: map[string]string{
				"app": instance.Name,
			},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector: map[string]string{
				"app": instance.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Protocol: corev1.ProtocolTCP,
					Port:     4369,
					Name:     "epmd",
				},
			},
		},
	}
}

func GenerateIngressService(instance rabbitmqv1beta1.RabbitmqCluster, serviceType string, serviceAnnotations map[string]string) *corev1.Service {
	if instance.Spec.Service.Type != "" {
		serviceType = instance.Spec.Service.Type
	} else if serviceType == "" {
		serviceType = "ClusterIP"
	}

	if instance.Spec.Service.Annotations != nil {
		serviceAnnotations = instance.Spec.Service.Annotations
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.ChildResourceName("rabbitmq-ingress"),
			Namespace: instance.Namespace,
			Labels: map[string]string{
				"app": instance.Name,
			},
			Annotations: serviceAnnotations,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceType(serviceType),
			Selector: map[string]string{
				"app": instance.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Protocol: corev1.ProtocolTCP,
					Port:     5672,
					Name:     "amqp",
				},
				{
					Protocol: corev1.ProtocolTCP,
					Port:     15672,
					Name:     "http",
				},
				{
					Protocol: corev1.ProtocolTCP,
					Port:     15692,
					Name:     "prometheus",
				},
			},
		},
	}
}
