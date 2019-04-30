package broker

import (
	"context"
	"errors"
	"fmt"

	"github.com/pivotal-cf/brokerapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RabbitMQServiceBroker struct {
	Config Config
}

func New(cfg Config) brokerapi.ServiceBroker {
	return RabbitMQServiceBroker{
		Config: cfg,
	}
}

func (rabbitmqServiceBroker RabbitMQServiceBroker) Deprovision(ctx context.Context, instanceID string, details brokerapi.DeprovisionDetails, asyncAllowed bool) (brokerapi.DeprovisionServiceSpec, error) {
	return brokerapi.DeprovisionServiceSpec{}, errors.New("Not implemented")
}

func (rabbitmqServiceBroker RabbitMQServiceBroker) GetInstance(ctx context.Context, instanceID string) (brokerapi.GetInstanceDetailsSpec, error) {
	return brokerapi.GetInstanceDetailsSpec{}, errors.New("Not implemented")
}

func (rabbitmqServiceBroker RabbitMQServiceBroker) Update(ctx context.Context, instanceID string, details brokerapi.UpdateDetails, asyncAllowed bool) (brokerapi.UpdateServiceSpec, error) {
	return brokerapi.UpdateServiceSpec{}, errors.New("Not implemented")
}

func (rabbitmqServiceBroker RabbitMQServiceBroker) LastOperation(ctx context.Context, instanceID string, details brokerapi.PollDetails) (brokerapi.LastOperation, error) {
	kubernetesClient, err := createKubernetesClient()
	if err != nil {
		return brokerapi.LastOperation{State: brokerapi.Failed}, fmt.Errorf("Failed to create kubernetes client: %s", err)
	}

	service, err := kubernetesClient.CoreV1().Services("rabbitmq-for-kubernetes").Get(fmt.Sprintf("p-%s-rabbitmq", instanceID), metav1.GetOptions{})
	if err != nil {
		return brokerapi.LastOperation{State: brokerapi.InProgress}, fmt.Errorf("Service still provisioning: %s", err)
	}

	if len(service.Status.LoadBalancer.Ingress) == 0 {
		return brokerapi.LastOperation{State: brokerapi.InProgress}, errors.New("Service external IP still provisioning")
	}

	return brokerapi.LastOperation{State: brokerapi.Succeeded}, nil
}

// func (rabbitmqServiceBroker RabbitMQServiceBroker) Bind(ctx context.Context, instanceID, bindingID string, details brokerapi.BindDetails, asyncAllowed bool) (brokerapi.Binding, error) {
// 	return brokerapi.Binding{}, errors.New("Not implemented")
// }

func (rabbitmqServiceBroker RabbitMQServiceBroker) Unbind(ctx context.Context, instanceID, bindingID string, details brokerapi.UnbindDetails, asyncAllowed bool) (brokerapi.UnbindSpec, error) {
	return brokerapi.UnbindSpec{}, errors.New("Not implemented")
}

func (rabbitmqServiceBroker RabbitMQServiceBroker) GetBinding(ctx context.Context, instanceID, bindingID string) (brokerapi.GetBindingSpec, error) {
	return brokerapi.GetBindingSpec{}, errors.New("Not implemented")
}

func (rabbitmqServiceBroker RabbitMQServiceBroker) LastBindingOperation(ctx context.Context, instanceID, bindingID string, details brokerapi.PollDetails) (brokerapi.LastOperation, error) {
	return brokerapi.LastOperation{}, errors.New("Not implemented")
}
