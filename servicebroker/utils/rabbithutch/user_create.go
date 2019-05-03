package rabbithutch

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	rabbithole "github.com/michaelklishin/rabbit-hole"
	"github.com/pivotal-cf/brokerapi"
)

func (r *rabbitHutch) CreateUserAndGrantPermissions(username, vhost, tags string) (string, error) {
	if tags == "" {
		tags = "policymaker,management"
	}

	password, err := generatePassword()
	if err != nil {
		return "", err
	}

	userSettings := rabbithole.UserSettings{
		Password: password,
		Tags:     tags,
	}

	response, err := r.client.PutUser(username, userSettings)
	if err != nil {
		return "", err
	}
	if response != nil && response.StatusCode == http.StatusNoContent {
		return "", brokerapi.ErrBindingAlreadyExists
	}

	if err = r.AssignPermissionsTo(vhost, username); err != nil {
		r.DeleteUser(username)
		return "", err
	}

	return password, nil
}

func generatePassword() (string, error) {
	rb := make([]byte, 24)
	_, err := rand.Read(rb)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(rb), nil
}
