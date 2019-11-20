package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const expectedRabbitmqConf = `cluster_formation.peer_discovery_backend = rabbit_peer_discovery_k8s
cluster_formation.k8s.host = kubernetes.default.svc.cluster.local
cluster_formation.k8s.address_type = hostname
cluster_formation.node_cleanup.interval = 30
cluster_formation.node_cleanup.only_log_warning = true
cluster_partition_handling = pause_minority
queue_master_locator = min-masters`

var _ = Describe("GenerateServerConfigMap", func() {
	var (
		confMap  *corev1.ConfigMap
		instance = rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: v1.ObjectMeta{
				Name:      "a-name",
				Namespace: "a-namespace",
			},
		}

		cluster *resource.RabbitmqResourceBuilder
	)

	BeforeEach(func() {
		cluster = &resource.RabbitmqResourceBuilder{
			Instance: &instance,
		}
		confMap = cluster.ServerConfigMap()
	})

	It("generates a ConfigMap with the correct name and namespace", func() {
		Expect(confMap.Name).To(Equal(cluster.Instance.ChildResourceName("server-conf")))
		Expect(confMap.Namespace).To(Equal(cluster.Instance.Namespace))
	})

	It("generates a ConfigMap with required labels", func() {
		labels := confMap.Labels
		Expect(labels["app.kubernetes.io/name"]).To(Equal(cluster.Instance.Name))
		Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
		Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
	})

	It("generates a ConfigMap with required object fields", func() {
		expectedEnabledPlugins := "[" +
			"rabbitmq_management," +
			"rabbitmq_peer_discovery_k8s," +
			"rabbitmq_federation," +
			"rabbitmq_federation_management," +
			"rabbitmq_shovel," +
			"rabbitmq_shovel_management," +
			"rabbitmq_prometheus]."

		plugins, ok := confMap.Data["enabled_plugins"]
		Expect(ok).To(BeTrue())
		Expect(plugins).To(Equal(expectedEnabledPlugins))
	})

	It("generates a rabbitmq conf with the required configurations", func() {
		rabbitmqConf, ok := confMap.Data["rabbitmq.conf"]
		Expect(ok).To(BeTrue())
		Expect(rabbitmqConf).To(Equal(expectedRabbitmqConf))
	})

})
