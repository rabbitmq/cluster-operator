package resource

import (
	"fmt"

	"github.com/pivotal/rabbitmq-for-kubernetes/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (builder *RabbitmqResourceBuilder) IngressService() (*corev1.Service, error) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName("ingress"),
			Namespace: builder.Instance.Namespace,
			Labels:    metadata.Label(builder.Instance.Name),
		},
		Spec: corev1.ServiceSpec{
			Selector: metadata.LabelSelector(builder.Instance.Name),
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
	if err := controllerutil.SetControllerReference(builder.Instance, service, builder.DefaultConfiguration.Scheme); err != nil {
		return nil, fmt.Errorf("failed setting controller reference: %v", err)
	}

	builder.setServiceParams(service)
	builder.UpdateServiceParams(service)

	return service, nil
}

func (builder *RabbitmqResourceBuilder) setServiceParams(service *corev1.Service) {
	var serviceType string
	if builder.Instance.Spec.Service.Type != "" {
		serviceType = builder.Instance.Spec.Service.Type
	} else if builder.DefaultConfiguration.ServiceType != "" {
		serviceType = builder.DefaultConfiguration.ServiceType
	} else {
		serviceType = "ClusterIP"
	}
	service.Spec.Type = corev1.ServiceType(serviceType)

	service.Annotations = builder.DefaultConfiguration.ServiceAnnotations
}

func (builder *RabbitmqResourceBuilder) UpdateServiceParams(service *corev1.Service) {
	if builder.Instance.Spec.Service.Annotations != nil {
		service.Annotations = builder.Instance.Spec.Service.Annotations
	}
	builder.updateLabels(&service.ObjectMeta)
}
