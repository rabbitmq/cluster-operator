package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("HeadlessService", func() {
	var (
		instance       rabbitmqv1beta1.RabbitmqCluster
		cluster        *resource.RabbitmqResourceBuilder
		serviceBuilder *resource.HeadlessServiceBuilder
		service        *corev1.Service
	)

	BeforeEach(func() {
		instance = rabbitmqv1beta1.RabbitmqCluster{}
		instance.Namespace = "foo"
		instance.Name = "foo"
		cluster = &resource.RabbitmqResourceBuilder{
			Instance: &instance,
		}
		serviceBuilder = cluster.HeadlessService()
	})

	Context("Build", func() {
		BeforeEach(func() {
			obj, _ := serviceBuilder.Build()
			service = obj.(*corev1.Service)
		})
		It("generates a service object with the correct name", func() {
			Expect(service.Name).To(Equal(instance.ChildResourceName("headless")))
		})

		It("generates a service object with the correct namespace", func() {
			Expect(service.Namespace).To(Equal(instance.Namespace))
		})

		It("generates a service object with the correct label", func() {
			labels := service.Labels
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
			Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
		})

		It("generates a service object with the correct selector", func() {
			Expect(service.Spec.Selector["app.kubernetes.io/name"]).To(Equal(instance.Name))
		})

		It("generates a headless service object", func() {
			Expect(service.Spec.ClusterIP).To(Equal("None"))
		})

		It("generates a service object with the right ports exposed", func() {
			epmdPort := corev1.ServicePort{
				Name:     "epmd",
				Port:     4369,
				Protocol: corev1.ProtocolTCP,
			}
			Expect(service.Spec.Ports).Should(ConsistOf(epmdPort))
		})

	})

	Context("Build with labels on CR", func() {
		BeforeEach(func() {
			instance.Labels = map[string]string{
				"app.kubernetes.io/foo": "bar",
				"foo":                   "bar",
				"rabbitmq":              "is-great",
				"foo/app.kubernetes.io": "edgecase",
			}
			obj, _ := serviceBuilder.Build()
			service = obj.(*corev1.Service)
		})

		It("has the labels from the CRD on the headless service", func() {
			testLabels(service.Labels)
		})
	})

	Context("Update", func() {
		BeforeEach(func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{}
			instance.Namespace = "rabbit-labelled"
			instance.Name = "rabbit-labelled"
			instance.Labels = map[string]string{
				"app.kubernetes.io/foo": "bar",
				"foo":                   "bar",
				"rabbitmq":              "is-great",
				"foo/app.kubernetes.io": "edgecase",
			}

			cluster = &resource.RabbitmqResourceBuilder{
				Instance: &instance,
			}

			service = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name": "rabbit-labelled",
					},
				},
			}
			err := serviceBuilder.Update(service)
			Expect(err).NotTo(HaveOccurred())
		})

		It("adds labels from the CRD on the headless service", func() {
			testLabels(service.Labels)
		})

		It("persists the labels it had before Update", func() {
			Expect(service.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "rabbit-labelled"))
		})
	})
})
