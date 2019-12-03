package resource

import (
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	headlessServiceName = "headless"
)

func (builder *RabbitmqResourceBuilder) HeadlessService() *HeadlessServiceBuilder {
	return &HeadlessServiceBuilder{
		Instance:             builder.Instance,
		DefaultConfiguration: builder.DefaultConfiguration,
	}
}

type HeadlessServiceBuilder struct {
	Instance             *rabbitmqv1beta1.RabbitmqCluster
	DefaultConfiguration DefaultConfiguration
}

func (builder *HeadlessServiceBuilder) Update(service runtime.Object) error {
	updateLabels(&service.(*corev1.Service).ObjectMeta, builder.Instance.Labels)
	return nil
}

func (builder *HeadlessServiceBuilder) Build() (runtime.Object, error) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName(headlessServiceName),
			Namespace: builder.Instance.Namespace,
			Labels:    metadata.Label(builder.Instance.Name),
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector:  metadata.LabelSelector(builder.Instance.Name),
			Ports: []corev1.ServicePort{
				{
					Protocol: corev1.ProtocolTCP,
					Port:     4369,
					Name:     "epmd",
				},
			},
		},
	}

	updateLabels(&service.ObjectMeta, builder.Instance.Labels)

	return service, nil
}
