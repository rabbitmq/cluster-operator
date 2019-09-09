package resource

import (
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConfigMapSuffix string = "-rabbitmq-plugins"
)

func GenerateConfigMap(instance rabbitmqv1beta1.RabbitmqCluster) *corev1.ConfigMap {

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + ConfigMapSuffix,
			Namespace: instance.Namespace,
			Labels: map[string]string{
				"app":             "pivotal-rabbitmq",
				"RabbitmqCluster": instance.Name,
			},
		},
		Data: map[string]string{
			"enabled_plugins": "[" +
				"rabbitmq_management," +
				"rabbitmq_peer_discovery_k8s," +
				"rabbitmq_federation," +
				"rabbitmq_federation_management," +
				"rabbitmq_shovel," +
				"rabbitmq_shovel_management," +
				"rabbitmq_prometheus].",
		},
	}
}
