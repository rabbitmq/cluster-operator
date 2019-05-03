package broker

import (
	"context"
	"fmt"

	"servicebroker/utils/rabbithutch"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rabbithole "github.com/michaelklishin/rabbit-hole"
	"github.com/pivotal-cf/brokerapi"
)

func (rabbitmqServiceBroker RabbitMQServiceBroker) Unbind(ctx context.Context, instanceID, bindingID string, details brokerapi.UnbindDetails, asyncAllowed bool) (brokerapi.UnbindSpec, error) {
	username := bindingID

	kubernetesClient, err := createKubernetesClient()
	if err != nil {
		return brokerapi.UnbindSpec{}, fmt.Errorf("Failed to create kubernetes client: %s", err)
	}

	getOptions := metav1.GetOptions{}
	service, err := kubernetesClient.CoreV1().Services("rabbitmq-for-kubernetes").Get(fmt.Sprintf("p-%s-rabbitmq", instanceID), getOptions)
	if err != nil {
		return brokerapi.UnbindSpec{}, fmt.Errorf("Failed to retrieve service: %s", err)
	}

	var serviceIP string
	if len(service.Status.LoadBalancer.Ingress) > 0 {
		serviceIP = service.Status.LoadBalancer.Ingress[0].IP
	} else {
		return brokerapi.UnbindSpec{}, fmt.Errorf("Failed to retrieve service IP for %s", service.Name)
	}

	client, _ := rabbithole.NewClient(
		fmt.Sprintf("http://%s:15672", serviceIP),
		rabbitmqServiceBroker.Config.RabbitMQ.Administrator.Username,
		rabbitmqServiceBroker.Config.RabbitMQ.Administrator.Password,
	)

	rabbit := rabbithutch.New(client)
	err = rabbit.DeleteUserAndConnections(username)
	if err != nil {
		return brokerapi.UnbindSpec{}, err
	}

	return brokerapi.UnbindSpec{}, nil
}
