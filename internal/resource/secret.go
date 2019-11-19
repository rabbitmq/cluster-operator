package resource

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const (
	imageSecretSuffix = "registry-access"
)

func (cluster *RabbitmqCluster) RegistrySecret() *corev1.Secret {
	registrySecret := &corev1.Secret{}

	registrySecret.Namespace = cluster.Instance.Namespace
	registrySecret.Name = RegistrySecretName(cluster.Instance.Name)
	registrySecret.Data = cluster.DefaultConfiguration.OperatorRegistrySecret.Data
	registrySecret.Type = cluster.DefaultConfiguration.OperatorRegistrySecret.Type
	return registrySecret
}

func RegistrySecretName(instanceName string) string {
	return fmt.Sprintf("%s-%s", instanceName, imageSecretSuffix)
}
