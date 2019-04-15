package broker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/brokerapi"
	rmq "github.com/pivotal/rabbitmq-for-kubernetes/pkg/apis/rabbitmq/v1beta1"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/pkg/client/clientset/versioned/typed/rabbitmq/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func (rabbitmqServiceBroker RabbitMQServiceBroker) Provision(ctx context.Context, instanceID string, details brokerapi.ProvisionDetails, asyncAllowed bool) (brokerapi.ProvisionedServiceSpec, error) {

	spec, err := rabbitmqServiceBroker.GenerateSpec(instanceID, details)
	if err != nil {
		return brokerapi.ProvisionedServiceSpec{}, fmt.Errorf("Failed to create in-cluster config: %s", err)
	}

	var config *rest.Config
	if len(os.Getenv("KUBERNETES_SERVICE_HOST")) > 0 {
		// creates the in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return brokerapi.ProvisionedServiceSpec{}, fmt.Errorf("Failed to create in-cluster config: %s", err)
		}
	} else {
		var kubeconfig string
		if len(os.Getenv("KUBECONFIG")) > 0 {
			kubeconfig = os.Getenv("KUBECONFIG")
		} else {
			kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube/config")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return brokerapi.ProvisionedServiceSpec{}, fmt.Errorf("Failed to create out of cluster config: %s", err)
		}
	}

	// creates the clientset
	clientset, err := rabbitmqv1beta1.NewForConfig(config)
	if err != nil {
		return brokerapi.ProvisionedServiceSpec{}, fmt.Errorf("Failed to create clientset from config: %s", err)
	}

	_, err = clientset.RabbitmqClusters(spec.ObjectMeta.Namespace).Create(&spec)
	if err != nil {
		return brokerapi.ProvisionedServiceSpec{}, fmt.Errorf("Failed to create RMQ cluster: %s", err)
	}

	return brokerapi.ProvisionedServiceSpec{}, nil
}

func (rabbitMQServiceBroker RabbitMQServiceBroker) GenerateSpec(instanceID string, details brokerapi.ProvisionDetails) (spec rmq.RabbitmqCluster, err error) {

	var planName string

	for _, p := range rabbitMQServiceBroker.Config.ServiceCatalog.Plans {
		if p.ID == details.PlanID {
			planName = p.Name
			break
		}
	}

	//TODO: read plans from rabbitmqServiceBroker.Config
	if planName != "single" && planName != "ha" {
		return rmq.RabbitmqCluster{}, fmt.Errorf("Unknown plan ID %s", details.PlanID)
	}

	spec = rmq.RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instanceID,
			Namespace: "rabbitmq-for-kubernetes",
		},
		Spec: rmq.RabbitmqClusterSpec{
			Plan: planName,
		},
	}

	return spec, nil

}
