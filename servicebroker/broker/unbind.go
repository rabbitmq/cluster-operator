package broker

import (
	"context"

	"github.com/pivotal-cf/brokerapi"
)

func (rabbitmqServiceBroker RabbitMQServiceBroker) Unbind(ctx context.Context, instanceID, bindingID string, details brokerapi.UnbindDetails, asyncAllowed bool) (brokerapi.UnbindSpec, error) {
	return brokerapi.UnbindSpec{}, nil
}
