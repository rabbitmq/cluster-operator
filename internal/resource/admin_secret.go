package resource

import (
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	adminSecretName = "admin"
)

func (builder *RabbitmqResourceBuilder) AdminSecret() (*corev1.Secret, error) {
	username, err := randomEncodedString(24)
	if err != nil {
		return nil, err
	}

	password, err := randomEncodedString(24)
	if err != nil {
		return nil, err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName(adminSecretName),
			Namespace: builder.Instance.Namespace,
			Labels:    metadata.Label(builder.Instance.Name),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"rabbitmq-username": []byte(username),
			"rabbitmq-password": []byte(password),
		},
	}

	builder.updateLabels(&secret.ObjectMeta)
	return secret, nil
}
