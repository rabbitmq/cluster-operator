package internal

import (
	"crypto/rand"
	"encoding/base64"
)

func RandomEncodedString(dataLen int) (string, error) {
	randomBytes := make([]byte, dataLen)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(randomBytes), nil
}
