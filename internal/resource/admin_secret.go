package resource

import (
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	adminSecretName = "admin"
)

type AdminSecretBuilder struct {
	Instance             *rabbitmqv1beta1.RabbitmqCluster
	DefaultConfiguration DefaultConfiguration
}

func (builder *RabbitmqResourceBuilder) AdminSecret() *AdminSecretBuilder {
	return &AdminSecretBuilder{
		Instance:             builder.Instance,
		DefaultConfiguration: builder.DefaultConfiguration,
	}
}

func (builder *AdminSecretBuilder) Update(object runtime.Object) error {
	updateLabels(&object.(*corev1.Secret).ObjectMeta, builder.Instance.Labels)
	return nil
}

func (builder *AdminSecretBuilder) Build() (runtime.Object, error) {
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

	updateLabels(&secret.ObjectMeta, builder.Instance.Labels)
	return secret, nil
}
