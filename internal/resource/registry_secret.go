package resource

import (
	"fmt"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	imageSecretSuffix = "registry-access"
)

type RegistrySecretBuilder struct {
	Instance             *rabbitmqv1beta1.RabbitmqCluster
	DefaultConfiguration DefaultConfiguration
}

func (builder *RabbitmqResourceBuilder) RegistrySecret() *RegistrySecretBuilder {
	return &RegistrySecretBuilder{
		Instance:             builder.Instance,
		DefaultConfiguration: builder.DefaultConfiguration,
	}
}

func (builder *RegistrySecretBuilder) Update(object runtime.Object) error {
	secret := object.(*corev1.Secret)
	secret.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)
	secret.Annotations = metadata.GetAnnotations(builder.Instance.Annotations)
	return nil
}

func (builder *RegistrySecretBuilder) Build() (runtime.Object, error) {
	if builder.DefaultConfiguration.OperatorRegistrySecret == nil {
		return nil, nil
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        RegistrySecretName(builder.Instance.Name),
			Namespace:   builder.Instance.Namespace,
			Labels:      metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels),
			Annotations: metadata.GetAnnotations(builder.Instance.Annotations),
		},
		Data: builder.DefaultConfiguration.OperatorRegistrySecret.Data,
		Type: builder.DefaultConfiguration.OperatorRegistrySecret.Type,
	}, nil
}

func RegistrySecretName(instanceName string) string {
	return fmt.Sprintf("%s-%s", instanceName, imageSecretSuffix)
}
