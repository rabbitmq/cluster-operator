package main

import (
	"encoding/json"
	"io/ioutil"
	"log"

	"github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
)

func main() {
	rabbitmqCluster := v1beta1.RabbitmqCluster{}
	rabbitmqClusterWithDefaults := v1beta1.MergeDefaults(rabbitmqCluster)
	rabbitmqClusterWithDefaults.TypeMeta.Kind = "RabbitmqCluster"
	rabbitmqClusterWithDefaults.Spec.Image = "rabbitmq"
	rabbitmqClusterWithDefaults.Spec.ImagePullSecret = "my-secret"
	rabbitmqClusterWithDefaults.APIVersion = "rabbitmq.pivotal.io/v1beta1"
	rabbitmqClusterWithDefaults.Spec.Affinity = &corev1.Affinity{}
	rabbitmqClusterWithDefaults.Spec.Tolerations = []corev1.Toleration{{}}

	j, err := json.Marshal(rabbitmqClusterWithDefaults)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	var jsonInterface interface{}
	if err := yaml.Unmarshal(j, &jsonInterface); err != nil {
		log.Fatalf("error: %v", err)
	}

	y, err := yaml.Marshal(jsonInterface)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	err = ioutil.WriteFile("./cr-example.yaml", y, 0644)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
}
