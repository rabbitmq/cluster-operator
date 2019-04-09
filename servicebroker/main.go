/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Note: the example only works with the code within the same release/branch.
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

//func main() {
//	// creates the in-cluster config
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
