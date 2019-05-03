package rabbithutch

import (
	rabbithole "github.com/michaelklishin/rabbit-hole"
)

func (r *rabbitHutch) CreatePolicy(vhost, name string, priority int, definition map[string]interface{}) error {
	policy := rabbithole.Policy{
		Definition: rabbithole.PolicyDefinition(definition),
		Priority:   priority,
		Vhost:      vhost,
		Pattern:    ".*",
		ApplyTo:    "all",
		Name:       name,
	}

	err := validateResponse(r.client.PutPolicy(vhost, name, policy))
	if err != nil {
		return err
	}

	return nil
}
