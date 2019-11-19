package resource

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const (
	imageSecretSuffix = "registry-access"
)

func (cluster *RabbitmqCluster) RegistrySecret(secret *corev1.Secret) *corev1.Secret {
	registrySecret := &corev1.Secret{}

	registrySecret.Namespace = cluster.Instance.Namespace
	registrySecret.Name = fmt.Sprintf("%s-%s", cluster.Instance.Name, imageSecretSuffix)
	registrySecret.Data = secret.Data
	registrySecret.Type = secret.Type
	return registrySecret
}
