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
cluster_partition_handling = autoheal
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
	)

	BeforeEach(func() {
		confMap = resource.GenerateServerConfigMap(instance)
	})

	It("generates a ConfigMap with the correct name and namespace", func() {
		Expect(confMap.Name).To(Equal(instance.ChildResourceName("server-conf")))
		Expect(confMap.Namespace).To(Equal(instance.Namespace))
	})

	It("generates a ConfigMap with required labels", func() {
		Expect(confMap.Labels["app"]).To(Equal("pivotal-rabbitmq"))
		Expect(confMap.Labels["RabbitmqCluster"]).To(Equal(instance.Name))
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
