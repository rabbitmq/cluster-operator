package resource

import (
	"fmt"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (builder *RabbitmqResourceBuilder) IngressService() *IngressServiceBuilder {
	return &IngressServiceBuilder{
		Instance: builder.Instance,
		Scheme:   builder.Scheme,
	}
}

type IngressServiceBuilder struct {
	Instance *rabbitmqv1beta1.RabbitmqCluster
	Scheme   *runtime.Scheme
}

func (builder *IngressServiceBuilder) Build() (runtime.Object, error) {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName("ingress"),
			Namespace: builder.Instance.Namespace,
		},
	}, nil
}

func (builder *IngressServiceBuilder) Update(object runtime.Object) error {
	service := object.(*corev1.Service)
	builder.setAnnotations(service)
	service.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)
	service.Spec.Type = corev1.ServiceType(builder.Instance.Spec.Service.Type)
	service.Spec.Selector = metadata.LabelSelector(builder.Instance.Name)

	service.Spec.Ports = updatePorts(service.Spec.Ports)

	if builder.Instance.Spec.Service.Type == "ClusterIP" || builder.Instance.Spec.Service.Type == "" {
		for i := range service.Spec.Ports {
			service.Spec.Ports[i].NodePort = int32(0)
		}
	}

	if err := controllerutil.SetControllerReference(builder.Instance, service, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %v", err)
	}

	return nil
}

func updatePorts(servicePorts []corev1.ServicePort) []corev1.ServicePort {
	servicePortsMap := map[string]corev1.ServicePort{
		"amqp": corev1.ServicePort{
			Protocol: corev1.ProtocolTCP,
			Port:     5672,
			Name:     "amqp",
		},
		"amqp-tls": corev1.ServicePort{
			Protocol: corev1.ProtocolTCP,
			Port:     5671,
			Name:     "amqp-tls",
		},
		"management": corev1.ServicePort{
			Protocol: corev1.ProtocolTCP,
			Port:     15672,
			Name:     "management",
		},
		"prometheus": corev1.ServicePort{
			Protocol: corev1.ProtocolTCP,
			Port:     15692,
			Name:     "prometheus",
		},
	}
	updatedServicePorts := []corev1.ServicePort{}

	for _, servicePort := range servicePorts {
		if value, exists := servicePortsMap[servicePort.Name]; exists {
			value.NodePort = servicePort.NodePort

			updatedServicePorts = append(updatedServicePorts, value)
			delete(servicePortsMap, servicePort.Name)
		}
	}

	for _, value := range servicePortsMap {
		updatedServicePorts = append(updatedServicePorts, value)
	}

	return updatedServicePorts

}

func (builder *IngressServiceBuilder) setAnnotations(service *corev1.Service) {
	if builder.Instance.Spec.Service.Annotations != nil {
		service.Annotations = metadata.ReconcileAnnotations(service.Annotations, builder.Instance.Annotations, builder.Instance.Spec.Service.Annotations)
	} else {
		service.Annotations = metadata.ReconcileAnnotations(service.Annotations, builder.Instance.Annotations)
	}
}
