package rabbithutch

import (
	rabbithole "github.com/michaelklishin/rabbit-hole"
)

func (r *rabbitHutch) AssignPermissionsTo(vhost, username string) error {
	permissions := rabbithole.Permissions{Configure: ".*", Write: ".*", Read: ".*"}
	return validateResponse(r.client.UpdatePermissionsIn(vhost, username, permissions))
}
