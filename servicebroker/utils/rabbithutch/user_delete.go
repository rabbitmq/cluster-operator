package rabbithutch

import (
	"net/http"

	"github.com/pivotal-cf/brokerapi"
)

func (r *rabbitHutch) DeleteUserAndConnections(username string) error {
	defer func() {
		conns, _ := r.client.ListConnections()
		for _, conn := range conns {
			if conn.User == username {
				r.client.CloseConnection(conn.Name)
			}
		}
	}()

	return r.DeleteUser(username)
}

func (r *rabbitHutch) DeleteUser(username string) error {
	resp, err := r.client.DeleteUser(username)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return brokerapi.ErrBindingDoesNotExist
	}

	if err := validateResponse(resp, err); err != nil {
		return err
	}
	return nil
}
