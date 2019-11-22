package resource

import (
	"fmt"

	"github.com/pivotal/rabbitmq-for-kubernetes/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (cluster *RabbitmqResourceBuilder) IngressService() (*corev1.Service, error) {
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
	if err := controllerutil.SetControllerReference(cluster.Instance, service, cluster.DefaultConfiguration.Scheme); err != nil {
		return nil, fmt.Errorf("failed setting controller reference: %v", err)
	}

	cluster.setServiceParams(service)
	cluster.UpdateServiceParams(service)

	return service, nil
}

func (cluster *RabbitmqResourceBuilder) setServiceParams(service *corev1.Service) {
	var serviceType string
	if cluster.Instance.Spec.Service.Type != "" {
		serviceType = cluster.Instance.Spec.Service.Type
	} else if cluster.DefaultConfiguration.ServiceType != "" {
		serviceType = cluster.DefaultConfiguration.ServiceType
	} else {
		serviceType = "ClusterIP"
	}
	service.Spec.Type = corev1.ServiceType(serviceType)

	service.Annotations = cluster.DefaultConfiguration.ServiceAnnotations
}

func (cluster *RabbitmqResourceBuilder) UpdateServiceParams(service *corev1.Service) {
	if cluster.Instance.Spec.Service.Annotations != nil {
		service.Annotations = cluster.Instance.Spec.Service.Annotations
	}
}
