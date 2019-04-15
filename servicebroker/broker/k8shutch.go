//package broker
//
////go:generate counterfeiter . k8sHutch
//type K8sHutch interface {
//	Create(string) error
//}
package broker

import (
	"fmt"
	"net/http"
)

////go:generate counterfeiter -o ./fakes/rabbithutch_fake.go $FILE RabbitHutch
//
//type K8sBunny interface {
//	//VHostExists(string) (bool, error)
//	//VHostDelete(string) error
//	//VHostCreate(string) error
//	CreateUserAndGrantPermissions(string, string, string) (string, error)
//	//ProtocolPorts() (map[string]int, error)
//	//DeleteUserAndConnections(string) error
//	//DeleteUser(string) error
//	//UserList() ([]string, error)
//	//AssignPermissionsTo(string, string) error
//	//CreatePolicy(string, string, int, map[string]interface{}) error
//}

//type rabbitHutch struct {
//	client APIClient
//}

//func New(client APIClient) RabbitHutch {
//	return &rabbitHutch{client}
//}

func validateResponse(resp *http.Response, err error) error {
	if err != nil {
		return err
	}

	if resp != nil && (resp.StatusCode < http.StatusOK || resp.StatusCode > 299) {
		return fmt.Errorf("http request failed with status code: %v", resp.StatusCode)
	}

	return nil
}
