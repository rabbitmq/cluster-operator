package broker

import (
	"context"
	"fmt"

	"servicebroker/binding"

	"github.com/pivotal-cf/brokerapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func createKubernetesClient() (*kubernetes.Clientset, error) {
	config, err := clientsetConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func (broker RabbitMQServiceBroker) Bind(ctx context.Context, instanceID, bindingID string, details brokerapi.BindDetails, asyncAllowed bool) (brokerapi.Binding, error) {
	vhost := "%2f"

	kubernetesClient, err := createKubernetesClient()
	if err != nil {
		return brokerapi.Binding{}, fmt.Errorf("Failed to create kubernetes client: %s", err)
	}

	getOptions := metav1.GetOptions{}
	service, err := kubernetesClient.CoreV1().Services("rabbitmq-for-kubernetes").Get(fmt.Sprintf("p-%s-rabbitmq", instanceID), getOptions)
	if err != nil {
		return brokerapi.Binding{}, fmt.Errorf("Failed to retrieve service: %s", err)
	}

	var serviceIP string
	if len(service.Status.LoadBalancer.Ingress) > 0 {
		serviceIP = service.Status.LoadBalancer.Ingress[0].IP
	}

	credsBuilder := binding.Builder{
		MgmtDomain:    fmt.Sprintf("%s:%d", serviceIP, 15672),
		Hostnames:     []string{serviceIP},
		VHost:         vhost,
		Username:      broker.Config.RabbitMQ.Administrator.Username,
		Password:      broker.Config.RabbitMQ.Administrator.Password,
		TLS:           bool(broker.Config.RabbitMQ.TLS),
		ProtocolPorts: map[string]int{"amqp": 5672, "clustering": 25672, "http": 15672},
	}

	credentials, err := credsBuilder.Build()
	if err != nil {
		return brokerapi.Binding{}, err
	}

	return brokerapi.Binding{Credentials: credentials}, nil

}
