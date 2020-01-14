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
		Instance:             builder.Instance,
		DefaultConfiguration: builder.DefaultConfiguration,
	}
}

type IngressServiceBuilder struct {
	Instance             *rabbitmqv1beta1.RabbitmqCluster
	DefaultConfiguration DefaultConfiguration
}

func (builder *IngressServiceBuilder) Build() (runtime.Object, error) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName("ingress"),
			Namespace: builder.Instance.Namespace,
			Labels:    metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels),
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

	builder.setServiceType(service)
	builder.setAnnotations(service)

	return service, nil
}

func (builder *IngressServiceBuilder) setServiceType(service *corev1.Service) {
	var serviceType string
	if builder.Instance.Spec.Service.Type != "" {
		serviceType = builder.Instance.Spec.Service.Type
	} else if builder.DefaultConfiguration.ServiceType != "" {
		serviceType = builder.DefaultConfiguration.ServiceType
	} else {
		serviceType = "ClusterIP"
	}
	service.Spec.Type = corev1.ServiceType(serviceType)
}

func (builder *IngressServiceBuilder) Update(object runtime.Object) error {
	service := object.(*corev1.Service)
	builder.setAnnotations(service)
	service.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)
	return nil
}

func (builder *IngressServiceBuilder) setAnnotations(service *corev1.Service) {
	mergedAnnotations := map[string]string{}

	copyMap(mergedAnnotations, builder.Instance.Annotations)

	if builder.Instance.Spec.Service.Annotations != nil {
		copyMap(mergedAnnotations, builder.Instance.Spec.Service.Annotations)
	} else {
		copyMap(mergedAnnotations, builder.DefaultConfiguration.ServiceAnnotations)
	}

	service.Annotations = metadata.FilterAndJoinAnnotations(mergedAnnotations, nil)
}

func copyMap(destination, source map[string]string) {
	for k, v := range source {
		destination[k] = v
	}
}
