package resource

import (
	"strings"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	serverConfigMapName = "server-conf"
)

type ServerConfigMapBuilder struct {
	Instance             *rabbitmqv1beta1.RabbitmqCluster
	DefaultConfiguration DefaultConfiguration
}

func (builder *RabbitmqResourceBuilder) ServerConfigMap() *ServerConfigMapBuilder {
	return &ServerConfigMapBuilder{
		Instance:             builder.Instance,
		DefaultConfiguration: builder.DefaultConfiguration,
	}
}

func (builder *ServerConfigMapBuilder) Update(object runtime.Object) error {
	updateLabels(&object.(*corev1.ConfigMap).ObjectMeta, builder.Instance.Labels)
	return nil
}

func (builder *ServerConfigMapBuilder) Build() (runtime.Object, error) {
	serverConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName(serverConfigMapName),
			Namespace: builder.Instance.Namespace,
			Labels:    metadata.Label(builder.Instance.Name),
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

			"rabbitmq.conf": strings.Join([]string{
				"cluster_formation.peer_discovery_backend = rabbit_peer_discovery_k8s",
				"cluster_formation.k8s.host = kubernetes.default.svc.cluster.local",
				"cluster_formation.k8s.address_type = hostname",
				"cluster_formation.node_cleanup.interval = 30",
				"cluster_formation.node_cleanup.only_log_warning = true",
				"cluster_partition_handling = pause_minority",
				"queue_master_locator = min-masters",
			}, "\n"),
		},
	}
	updateLabels(&serverConfig.ObjectMeta, builder.Instance.Labels)
	return serverConfig, nil
}
