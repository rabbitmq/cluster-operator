package secret

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/pkg/apis/rabbitmq/v1beta1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const stringLen = 24

//go:generate counterfeiter . Secret

type Secret interface {
	New(instance *rabbitmqv1beta1.RabbitmqCluster) (*v1.Secret, error)
}

type RabbitSecret struct{}

func (r *RabbitSecret) New(instance *rabbitmqv1beta1.RabbitmqCluster) (*v1.Secret, error) {
	cookie, err := generate()
	if err != nil {
		return nil, err
	}
	secret := &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		Data: map[string][]byte{
			"erlang-cookie": cookie,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels: map[string]string{
				"instance": instance.Name,
			},
		},
		Type: "Opaque",
	}

	return secret, nil
}

func generate() ([]byte, error) {
	encoding := base64.RawURLEncoding

	randomBytes := make([]byte, encoding.DecodedLen(stringLen))
	if _, err := rand.Read(randomBytes); err != nil {
		return []byte{}, fmt.Errorf("reading random bytes failed:. %s", err)
	}

	return []byte(strings.TrimPrefix(encoding.EncodeToString(randomBytes), "-")), nil
}
