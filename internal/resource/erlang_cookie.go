package resource

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/pivotal/rabbitmq-for-kubernetes/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	erlangCookieName = "erlang-cookie"
)

func (cluster *RabbitmqCluster) ErlangCookie() (*corev1.Secret, error) {
	cookie, err := randomEncodedString(24)
	if err != nil {
		return nil, err
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Instance.ChildResourceName(erlangCookieName),
			Namespace: cluster.Instance.Namespace,
			Labels:    metadata.Label(cluster.Instance.Name),
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
