package broker

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal-cf/brokerapi"
)

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
