package resource

import (
	"fmt"
	"strings"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (builder *RabbitmqResourceBuilder) IngressService() IngressServiceBuilder {
	return IngressServiceBuilder{
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
	builder.Update(service)

	return service, nil
}

func (builder *IngressServiceBuilder) setServiceParams(service *corev1.Service) {
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

func (builder *IngressServiceBuilder) Update(object runtime.Object) {
	if builder.Instance.Spec.Service.Annotations != nil {
		object.(*corev1.Service).Annotations = builder.Instance.Spec.Service.Annotations
	}
	updateLabels(&object.(*corev1.Service).ObjectMeta, builder.Instance.Labels)
}

func (builder *IngressServiceBuilder) updateLabels(objectMeta *metav1.ObjectMeta) {
	if builder.Instance.Labels != nil {
		if objectMeta.Labels == nil {
			objectMeta.Labels = make(map[string]string)
		}
		for label, value := range builder.Instance.Labels {
			if !strings.HasPrefix(label, "app.kubernetes.io") {
				// TODO if a label is in the StatefulSet and in the CR, the value in the CR will overwrite the value in STS
				objectMeta.Labels[label] = value
			}
		}
	}
}
