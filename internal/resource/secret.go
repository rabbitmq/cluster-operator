package resource

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const (
	imageSecretSuffix = "registry-access"
)

func GenerateRegistrySecret(secret *corev1.Secret, namespace string, instanceName string) *corev1.Secret {
	registrySecret := &corev1.Secret{}

	registrySecret.Namespace = namespace
	registrySecret.Name = fmt.Sprintf("%s-%s", instanceName, imageSecretSuffix)
	registrySecret.Data = secret.Data
	registrySecret.Type = secret.Type
	return registrySecret
}
