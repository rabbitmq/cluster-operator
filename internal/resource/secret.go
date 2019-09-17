package resource

import (
	"crypto/rand"
	"encoding/base64"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	secretName        = "server"
	secretUsernameKey = "username"
	secretPasswordKey = "password"
	secretCookieKey   = "cookie"
)

func GenerateSecret(instance rabbitmqv1beta1.RabbitmqCluster) (*corev1.Secret, error) {
	username, err := randomEncodedString(24)
	if err != nil {
		return nil, err
	}

	password, err := randomEncodedString(24)
	if err != nil {
		return nil, err
	}

	cookie, err := randomEncodedString(24)
	if err != nil {
		return nil, err
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.ChildResourceName(secretName),
			Namespace: instance.Namespace,
			Labels: map[string]string{
				"app":             "pivotal-rabbitmq",
				"RabbitmqCluster": instance.Name,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			secretUsernameKey: []byte(username),
			secretPasswordKey: []byte(password),
			secretCookieKey:   []byte(cookie),
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
