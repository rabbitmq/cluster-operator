package rabbithutch

import (
	rabbithole "github.com/michaelklishin/rabbit-hole"
)

func (r *rabbitHutch) VHostCreate(vhost string) error {
	return validateResponse(r.client.PutVhost(vhost, rabbithole.VhostSettings{}))
}
