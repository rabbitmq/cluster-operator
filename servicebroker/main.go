package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"servicebroker/broker"

	"code.cloudfoundry.org/lager"
	"github.com/pivotal-cf/brokerapi"
)

var (
	configPath string
	port       int
)

func init() {
	flag.StringVar(&configPath, "configPath", "", "Config file location")
}

func main() {
	flag.Parse()

	logger := lager.NewLogger("rabbitmq-multitenant-go-broker")

	config, err := broker.ReadConfig(configPath)
	if err != nil {
		logger.Fatal("read-config", err)
	}

	broker := broker.New(config)
	credentials := brokerapi.BrokerCredentials{
		Username: config.Broker.Username,
		Password: config.Broker.Password,
	}

	brokerAPI := brokerapi.New(broker, logger, credentials)
	port := config.Broker.Port
	http.Handle("/", brokerAPI)
	fmt.Printf("RabbitMQ Service Broker listening on port %d\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

//func previous_main() {
//	//	// creates the in-cluster config
//	config, err := rest.InClusterConfig()
//	if err != nil {
//		panic(err.Error())
//	}
//	// creates the clientset
//	clientset, err := rabbitmqv1beta1.NewForConfig(config)
//	if err != nil {
//		panic(err.Error())
//	}
//
//	rmq := &rmq.RabbitmqCluster{
//		ObjectMeta: metav1.ObjectMeta{
//			Name:      "foo",
//			Namespace: "default",
//		},
//		Spec: rmq.RabbitmqClusterSpec{
//			Plan: "single",
//		},
//	}
//	_, err = clientset.RabbitmqClusters("default").Create(rmq)
//	if err != nil {
//		fmt.Print(err)
//	}
//}
