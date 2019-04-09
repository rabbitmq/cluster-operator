package broker

import (
	"context"

	"github.com/pivotal-cf/brokerapi"
)

func (b RabbitMQServiceBroker) Services(ctx context.Context) ([]brokerapi.Service, error) {
	var plans []brokerapi.ServicePlan
	for _, plan := range b.Config.ServiceCatalog.Plans {
		plans = append(plans, brokerapi.ServicePlan{
			ID:          plan.ID,
			Name:        plan.Name,
			Description: plan.Description,
		})
	}

	return []brokerapi.Service{
		brokerapi.Service{
			ID:          b.Config.ServiceCatalog.ID,
			Name:        b.Config.ServiceCatalog.Name,
			Description: b.Config.ServiceCatalog.Description,
			Bindable:    true,
			Plans:       plans,
		},
	}, nil
}
