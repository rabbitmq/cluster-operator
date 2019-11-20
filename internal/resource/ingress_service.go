package resource

import (
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (cluster *RabbitmqResourceBuilder) IngressService() *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Instance.ChildResourceName("ingress"),
			Namespace: cluster.Instance.Namespace,
			Labels:    metadata.Label(cluster.Instance.Name),
		},
		Spec: corev1.ServiceSpec{
			Selector: metadata.LabelSelector(cluster.Instance.Name),
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

	cluster.SetMutableServiceParams(service)
	cluster.setImmutableServiceParams(service)

	return service
}

func (cluster *RabbitmqResourceBuilder) SetMutableServiceParams(service *corev1.Service) {
	var serviceAnnotations map[string]string
	if cluster.Instance.Spec.Service.Annotations != nil {
		serviceAnnotations = cluster.Instance.Spec.Service.Annotations
	} else {
		serviceAnnotations = cluster.DefaultConfiguration.ServiceAnnotations
	}
	service.Annotations = serviceAnnotations
}

func (cluster *RabbitmqResourceBuilder) setImmutableServiceParams(service *corev1.Service) {
	var serviceType string
	if cluster.Instance.Spec.Service.Type != "" {
		serviceType = cluster.Instance.Spec.Service.Type
	} else if cluster.DefaultConfiguration.ServiceType == "" {
		serviceType = "ClusterIP"
	} else {
		serviceType = cluster.DefaultConfiguration.ServiceType
	}

	service.Spec.Type = corev1.ServiceType(serviceType)
}
