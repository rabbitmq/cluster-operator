package resource

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	adminSecretName   = "admin"
	erlangCookieName  = "erlang-cookie"
	imageSecretSuffix = "registry-access"
)

func GenerateAdminSecret(instance rabbitmqv1beta1.RabbitmqCluster) (*corev1.Secret, error) {
	username, err := randomEncodedString(24)
	if err != nil {
		return nil, err
	}

	password, err := randomEncodedString(24)
	if err != nil {
		return nil, err
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.ChildResourceName(adminSecretName),
			Namespace: instance.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      instance.Name,
				"app.kubernetes.io/component": "rabbitmq",
				"app.kubernetes.io/part-of":   "pivotal-rabbitmq",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"rabbitmq-username": []byte(username),
			"rabbitmq-password": []byte(password),
		},
	}, nil
}

func GenerateRegistrySecret(secret *corev1.Secret, namespace string, instanceName string) *corev1.Secret {
	registrySecret := &corev1.Secret{}

	registrySecret.Namespace = namespace
	registrySecret.Name = fmt.Sprintf("%s-%s", instanceName, imageSecretSuffix)
	registrySecret.Data = secret.Data
	registrySecret.Type = secret.Type
	return registrySecret
}

func GenerateErlangCookie(instance rabbitmqv1beta1.RabbitmqCluster) (*corev1.Secret, error) {
	cookie, err := randomEncodedString(24)
	if err != nil {
		return nil, err
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.ChildResourceName(erlangCookieName),
			Namespace: instance.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      instance.Name,
				"app.kubernetes.io/component": "rabbitmq",
				"app.kubernetes.io/part-of":   "pivotal-rabbitmq",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			".erlang.cookie": []byte(cookie),
		},
	}, nil
}

func randomEncodedString(dataLen int) (string, error) {
	randomBytes := make([]byte, dataLen)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(randomBytes), nil
}
