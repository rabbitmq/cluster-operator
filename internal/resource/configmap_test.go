package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("ConfigMap", func() {
	var instance rabbitmqv1beta1.RabbitmqCluster
	var confMap *corev1.ConfigMap

	BeforeEach(func() {
		instance = rabbitmqv1beta1.RabbitmqCluster{}
		instance.Namespace = "foo"
		instance.Name = "foo"
		confMap = resource.GenerateConfigMap(instance)
	})

	Context("Creates a ConfigMap with minimum requirements", func() {
		It("with name that follows conventions", func() {
			expectedName := instance.Name + ConfigMapSuffix
			Expect(confMap.Name).To(Equal(expectedName))
		})

		It("with required labels", func() {
			Expect(confMap.Labels["app"]).To(Equal("pivotal-rabbitmq"))
			Expect(confMap.Labels["RabbitmqCluster"]).To(Equal(instance.Name))
		})

		It("with required object fields", func() {

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
	})
})
