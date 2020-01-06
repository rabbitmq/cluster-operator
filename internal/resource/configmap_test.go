package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		configMap        *corev1.ConfigMap
		instance         rabbitmqv1beta1.RabbitmqCluster
		configMapBuilder *resource.ServerConfigMapBuilder
		builder          *resource.RabbitmqResourceBuilder
	)

	BeforeEach(func() {
		instance = rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: v1.ObjectMeta{
				Name:      "a name",
				Namespace: "a namespace",
			},
		}
		builder = &resource.RabbitmqResourceBuilder{
			Instance: &instance,
		}
		configMapBuilder = builder.ServerConfigMap()
	})

	Context("Build with defaults", func() {
		BeforeEach(func() {
			obj, err := configMapBuilder.Build()
			configMap = obj.(*corev1.ConfigMap)
			Expect(err).NotTo(HaveOccurred())
		})

		It("generates a ConfigMap with the correct name and namespace", func() {
			Expect(configMap.Name).To(Equal(builder.Instance.ChildResourceName("server-conf")))
			Expect(configMap.Namespace).To(Equal(builder.Instance.Namespace))
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

			plugins, ok := configMap.Data["enabled_plugins"]
			Expect(ok).To(BeTrue())
			Expect(plugins).To(Equal(expectedEnabledPlugins))
		})

		It("generates a rabbitmq conf with the required configurations", func() {
			rabbitmqConf, ok := configMap.Data["rabbitmq.conf"]
			Expect(ok).To(BeTrue())
			Expect(rabbitmqConf).To(Equal(expectedRabbitmqConf))
		})

		It("only creates the required labels", func() {
			labels := configMap.Labels
			Expect(len(labels)).To(Equal(3))
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
			Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
		})
	})

	Context("Build with instance labels", func() {
		BeforeEach(func() {
			instance.Labels = map[string]string{
				"app.kubernetes.io/foo": "bar",
				"foo":                   "bar",
				"rabbitmq":              "is-great",
				"foo/app.kubernetes.io": "edgecase",
			}

			obj, err := configMapBuilder.Build()
			configMap = obj.(*corev1.ConfigMap)
			Expect(err).NotTo(HaveOccurred())
		})

		It("has the labels from the CRD on the configMap", func() {
			testLabels(configMap.Labels)
		})

		It("also has the required labels", func() {
			labels := configMap.Labels
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
			Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
		})
	})

	Context("Build with instance annotations", func() {
		BeforeEach(func() {
			instance.Annotations = map[string]string{
				"my-annotation":              "i-like-this",
				"kubernetes.io/name":         "i-do-not-like-this",
				"kubectl.kubernetes.io/name": "i-do-not-like-this",
				"k8s.io/name":                "i-do-not-like-this",
			}

			obj, err := configMapBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			configMap = obj.(*corev1.ConfigMap)
		})

		It("has the annotations from the instance on the configMap", func() {
			testAnnotations(configMap.Annotations)
		})
	})

	Context("Update", func() {
		BeforeEach(func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rabbit-labelled",
				},
			}
			instance.Labels = map[string]string{
				"app.kubernetes.io/foo": "bar",
				"foo":                   "bar",
				"rabbitmq":              "is-great",
				"foo/app.kubernetes.io": "edgecase",
			}

			configMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":      instance.Name,
						"app.kubernetes.io/part-of":   "pivotal-rabbitmq",
						"this-was-the-previous-label": "should-be-deleted",
					},
				},
			}
			err := configMapBuilder.Update(configMap)
			Expect(err).NotTo(HaveOccurred())
		})

		It("adds labels from the CR", func() {
			testLabels(configMap.Labels)
		})

		It("restores the default labels", func() {
			labels := configMap.Labels
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
			Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
		})

		It("deletes the labels that are removed from the CR", func() {
			Expect(configMap.Labels).NotTo(HaveKey("this-was-the-previous-label"))
		})
	})

	Context("Update with instance annotations", func() {
		BeforeEach(func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rabbit-labelled",
				},
			}
			instance.Annotations = map[string]string{
				"my-annotation":              "i-like-this",
				"kubernetes.io/name":         "i-do-not-like-this",
				"kubectl.kubernetes.io/name": "i-do-not-like-this",
				"k8s.io/name":                "i-do-not-like-this",
			}

			configMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"old-annotations": "old-value",
					},
				},
			}
			err := configMapBuilder.Update(configMap)
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates config map annotations", func() {
			testAnnotations(configMap.Annotations)
		})
	})
})
