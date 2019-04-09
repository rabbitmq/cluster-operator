package broker_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"

	. "servicebroker/broker"
)

var _ = Describe("Catalog", func() {
	It("returns a valid catalog", func() {
		cfg := Config{
			ServiceCatalog: ServiceCatalog{
				ID:          "00000000-0000-0000-0000-000000000000",
				Name:        "p-rabbitmq",
				Description: "this is a description",
				Plans: []Plan{
					Plan{
						ID:          "id-foo",
						Name:        "name-foo",
						Description: "desc-foo",
					},
					Plan{
						ID:          "id-foo2",
						Name:        "name-foo2",
						Description: "desc-foo2",
					},
				},
			},
		}

		broker := defaultServiceBroker(cfg)
		services, err := broker.Services(context.Background())
		Expect(err).NotTo(HaveOccurred())

		Expect(services).To(Equal([]brokerapi.Service{brokerapi.Service{
			ID:          cfg.ServiceCatalog.ID,
			Name:        cfg.ServiceCatalog.Name,
			Description: cfg.ServiceCatalog.Description,
			Bindable:    true,
			Plans: []brokerapi.ServicePlan{
				brokerapi.ServicePlan{
					ID:          "id-foo",
					Name:        "name-foo",
					Description: "desc-foo",
				},
				brokerapi.ServicePlan{
					ID:          "id-foo2",
					Name:        "name-foo2",
					Description: "desc-foo2",
				},
			},
		}}))
	})

})

func defaultServiceBroker(conf Config) brokerapi.ServiceBroker {
	return New(conf)
}
