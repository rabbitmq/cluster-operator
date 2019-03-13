package secret

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	resourcegenerator "github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/resourcegenerator"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const stringLen = 24

func generate() ([]byte, error) {
	encoding := base64.RawURLEncoding

	randomBytes := make([]byte, encoding.DecodedLen(stringLen))
	if _, err := rand.Read(randomBytes); err != nil {
		return []byte{}, fmt.Errorf("reading random bytes failed:. %s", err)
	}

	return []byte(strings.TrimPrefix(encoding.EncodeToString(randomBytes), "-")), nil
}

func New(generationContext resourcegenerator.GenerationContext) (*v1.Secret, error) {
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
			Name:      generationContext.InstanceName,
			Namespace: generationContext.Namespace,
			Labels: map[string]string{
				"instance": generationContext.InstanceName,
			},
		},
		Type: "Opaque",
	}

	return secret, nil
}
