package cookiegenerator

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
)

const stringLen = 24

func Generate() (string, error) {
	encoding := base64.RawURLEncoding

	randomBytes := make([]byte, encoding.DecodedLen(stringLen))
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("reading random bytes failed:. %s", err)
	}

	return strings.TrimPrefix(encoding.EncodeToString(randomBytes), "-"), nil
}
