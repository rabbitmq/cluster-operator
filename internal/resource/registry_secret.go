package resource

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const (
	imageSecretSuffix = "registry-access"
)

func (builder *RabbitmqResourceBuilder) RegistrySecret() *corev1.Secret {
	registrySecret := &corev1.Secret{}

	registrySecret.Namespace = builder.Instance.Namespace
	registrySecret.Name = RegistrySecretName(builder.Instance.Name)
	registrySecret.Data = builder.DefaultConfiguration.OperatorRegistrySecret.Data
	registrySecret.Type = builder.DefaultConfiguration.OperatorRegistrySecret.Type
	updateLabels(&registrySecret.ObjectMeta, builder.Instance.Labels)
	return registrySecret
}

func RegistrySecretName(instanceName string) string {
	return fmt.Sprintf("%s-%s", instanceName, imageSecretSuffix)
}
