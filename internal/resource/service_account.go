package resource

import (
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	serviceAccountName = "server"
)

type ServiceAccountBuilder struct {
	Instance             *rabbitmqv1beta1.RabbitmqCluster
	DefaultConfiguration DefaultConfiguration
}

func (builder *RabbitmqResourceBuilder) ServiceAccount() *ServiceAccountBuilder {
	return &ServiceAccountBuilder{
		Instance:             builder.Instance,
		DefaultConfiguration: builder.DefaultConfiguration,
	}
}

func (builder *ServiceAccountBuilder) Update(object runtime.Object) error {
	updateLabels(&object.(*corev1.ServiceAccount).ObjectMeta, builder.Instance.Labels)
	return nil
}

func (builder *ServiceAccountBuilder) Build() (runtime.Object, error) {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: builder.Instance.Namespace,
			Name:      builder.Instance.ChildResourceName(serviceAccountName),
			Labels:    metadata.Label(builder.Instance.Name),
		},
	}

	updateLabels(&serviceAccount.ObjectMeta, builder.Instance.Labels)
	return serviceAccount, nil
}
