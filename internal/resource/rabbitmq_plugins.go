package resource

import (
	"strings"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
)

var requiredPlugins = []string{
	"rabbitmq_peer_discovery_k8s", // required for clustering
	"rabbitmq_prometheus",         // enforce prometheus metrics
	"rabbitmq_management",
}

type RabbitMQPlugins struct {
	requiredPlugins   []string
	additionalPlugins []string
}

func NewRabbitMQPlugins(plugins []rabbitmqv1beta1.Plugin) RabbitMQPlugins {
	additionalPlugins := make([]string, len(plugins))
	for i := range additionalPlugins {
		additionalPlugins[i] = string(plugins[i])
	}

	return RabbitMQPlugins{
		requiredPlugins:   requiredPlugins,
		additionalPlugins: additionalPlugins,
	}
}

func (r *RabbitMQPlugins) DesiredPlugins() []string {
	allPlugins := append(r.requiredPlugins, r.additionalPlugins...)

	check := make(map[string]bool)
	enabledPlugins := make([]string, 0)
	for _, p := range allPlugins {
		if _, ok := check[p]; !ok {
			check[p] = true
			enabledPlugins = append(enabledPlugins, p)
		}
	}
	return enabledPlugins
}

func (r *RabbitMQPlugins) AsString(sep string) string {
	return strings.Join(r.DesiredPlugins(), sep)
}
