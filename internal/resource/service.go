package resource

import (
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenerateService(instance rabbitmqv1beta1.RabbitmqCluster) *corev1.Service {
	serviceType := corev1.ServiceTypeClusterIP
	if instance.Spec.Service.Type != "" {
		switch specifiedService := instance.Spec.Service.Type; specifiedService {
		case "LoadBalancer":
			serviceType = corev1.ServiceTypeLoadBalancer
		}
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "p-" + instance.Name,
			Namespace: instance.Namespace,
			Labels: map[string]string{
				"app": instance.Name,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: serviceType,
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
			},
		},
	}
}
