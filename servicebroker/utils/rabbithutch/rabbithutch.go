package rabbithutch

import (
	"fmt"
	"net/http"

	rabbithole "github.com/michaelklishin/rabbit-hole"
)

//go:generate counterfeiter -o ./fakes/api_client_fake.go $FILE APIClient

type APIClient interface {
	GetVhost(string) (*rabbithole.VhostInfo, error)
	PutVhost(string, rabbithole.VhostSettings) (*http.Response, error)
	UpdatePermissionsIn(vhost, username string, permissions rabbithole.Permissions) (res *http.Response, err error)
	PutPolicy(vhost, name string, policy rabbithole.Policy) (res *http.Response, err error)
	DeleteVhost(vhostname string) (res *http.Response, err error)
	DeleteUser(username string) (res *http.Response, err error)
	ListUsers() (users []rabbithole.UserInfo, err error)
	PutUser(string, rabbithole.UserSettings) (*http.Response, error)
	ProtocolPorts() (res map[string]rabbithole.Port, err error)
	ListConnections() (conns []rabbithole.ConnectionInfo, err error)
	CloseConnection(name string) (res *http.Response, err error)
}

//go:generate counterfeiter -o ./fakes/rabbithutch_fake.go $FILE RabbitHutch

type RabbitHutch interface {
	VHostExists(string) (bool, error)
	VHostDelete(string) error
	VHostCreate(string) error
	CreateUserAndGrantPermissions(string, string, string) (string, error)
	ProtocolPorts() (map[string]int, error)
	DeleteUserAndConnections(string) error
	DeleteUser(string) error
	UserList() ([]string, error)
	AssignPermissionsTo(string, string) error
	CreatePolicy(string, string, int, map[string]interface{}) error
}

type rabbitHutch struct {
	client APIClient
}

func New(client APIClient) RabbitHutch {
	return &rabbitHutch{client}
}

func validateResponse(resp *http.Response, err error) error {
	if err != nil {
		return err
	}

	if resp != nil && (resp.StatusCode < http.StatusOK || resp.StatusCode > 299) {
		return fmt.Errorf("http request failed with status code: %v", resp.StatusCode)
	}

	return nil
}
