package broker_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
)

var _ = Describe("Catalog", func() {
	It("returns a valid catalog", func() {

		broker := defaultServiceBroker()
		services, err := broker.Services(context.Background())
		Expect(err).NotTo(HaveOccurred())

		Expect(services).To(Equal([]brokerapi.Service{brokerapi.Service{
			ID:          defaultCfg.ServiceCatalog.ID,
			Name:        defaultCfg.ServiceCatalog.Name,
			Description: defaultCfg.ServiceCatalog.Description,
			Bindable:    true,
			Tags:        []string{"rabbitmq", "amqp"},
			Plans: []brokerapi.ServicePlan{
				brokerapi.ServicePlan{
					ID:          "11111111-1111-1111-1111-111111111111",
					Name:        "ha",
					Description: "HA RabbitMQ on K8s",
				},
				brokerapi.ServicePlan{
					ID:          "22222222-2222-2222-2222-222222222222",
					Name:        "single",
					Description: "Single-node RabbitMQ on K8s",
				},
			},
		}}))
	})

})
