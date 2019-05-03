package rabbithutch

import (
	"net/http"

	rabbithole "github.com/michaelklishin/rabbit-hole"
)

func (r *rabbitHutch) VHostExists(vhost string) (bool, error) {
	if _, err := r.client.GetVhost(vhost); err != nil {
		if rabbitErr, ok := err.(rabbithole.ErrorResponse); ok && rabbitErr.StatusCode == http.StatusNotFound {
			return false, nil
		}

		return false, err
	}

	return true, nil
}
