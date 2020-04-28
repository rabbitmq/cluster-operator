package resource

import (
	"crypto/rand"
	"encoding/base64"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	erlangCookieName = "erlang-cookie"
)

type ErlangCookieBuilder struct {
	Instance *rabbitmqv1beta1.RabbitmqCluster
}

func (builder *RabbitmqResourceBuilder) ErlangCookie() *ErlangCookieBuilder {
	return &ErlangCookieBuilder{
		Instance: builder.Instance,
	}
}

func (builder *ErlangCookieBuilder) Build() (runtime.Object, error) {
	cookie, err := randomEncodedString(24)
	if err != nil {
		return nil, err
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName(erlangCookieName),
			Namespace: builder.Instance.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			".erlang.cookie": []byte(cookie),
		},
	}, nil
}

func (builder *ErlangCookieBuilder) Update(object runtime.Object) error {
	secret := object.(*corev1.Secret)
	secret.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)
	secret.Annotations = metadata.ReconcileAndFilterAnnotations(secret.GetAnnotations(), builder.Instance.Annotations)
	return nil
}

func randomEncodedString(dataLen int) (string, error) {
	randomBytes := make([]byte, dataLen)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(randomBytes), nil
}
