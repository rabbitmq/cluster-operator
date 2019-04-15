package broker_test

import (
	"testing"

	. "servicebroker/broker"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
)

var (
	defaultCfg = Config{
		ServiceCatalog: ServiceCatalog{
			ID:          "00000000-0000-0000-0000-000000000000",
			Name:        "p-rabbitmq-k8s",
			Description: "RabbitMQ on K8s",
			Plans: []Plan{
				Plan{
					ID:          "11111111-1111-1111-1111-111111111111",
					Name:        "ha",
					Description: "HA RabbitMQ on K8s",
				},
				Plan{
					ID:          "22222222-2222-2222-2222-222222222222",
					Name:        "single",
					Description: "Single-node RabbitMQ on K8s",
				},
			},
		},
	}
)

func TestBroker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Broker Suite")
}

func defaultServiceBroker() brokerapi.ServiceBroker {
	return New(defaultCfg)
}
