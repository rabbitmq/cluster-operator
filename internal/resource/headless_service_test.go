package resource_test

import (
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("HeadlessService", func() {
	var (
		instance rabbitmqv1beta1.RabbitmqCluster
		cluster  *resource.RabbitmqResourceBuilder
		service  *corev1.Service
	)

	BeforeEach(func() {
		instance = rabbitmqv1beta1.RabbitmqCluster{}
		instance.Namespace = "foo"
		instance.Name = "foo"
		cluster = &resource.RabbitmqResourceBuilder{
			Instance: &instance,
		}
		serviceBuilder := cluster.HeadlessService()
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

	Context("label inheritance", func() {
		BeforeEach(func() {
			instance.Labels = map[string]string{
				"app.kubernetes.io/foo": "bar",
				"foo":                   "bar",
				"rabbitmq":              "is-great",
				"foo/app.kubernetes.io": "edgecase",
			}
		})

		It("has the labels from the CRD on the ingress service", func() {
			serviceBuilder := cluster.HeadlessService()
			obj, _ := serviceBuilder.Build()
			headlessService := obj.(*corev1.Service)
			testLabels(headlessService.Labels)
		})
	})

	Context("Update", func() {
		var (
			service        *corev1.Service
			serviceBuilder *resource.HeadlessServiceBuilder
		)

		BeforeEach(func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{}
			instance.Namespace = "foo"
			instance.Name = "foo"
			instance.Labels = map[string]string{
				"app.kubernetes.io/foo": "bar",
				"foo":                   "bar",
				"rabbitmq":              "is-great",
				"foo/app.kubernetes.io": "edgecase",
			}

			cluster = &resource.RabbitmqResourceBuilder{
				Instance: &instance,
			}

			serviceBuilder = cluster.HeadlessService()
			obj, _ := serviceBuilder.Build()
			service = obj.(*corev1.Service)
		})

		It("adds labels from the CRD on the headless service", func() {
			err := serviceBuilder.Update(service)
			Expect(err).NotTo(HaveOccurred())

			testLabels(service.Labels)
		})

		It("sets nothing if the instance has no labels", func() {
			serviceBuilder = cluster.HeadlessService()
			serviceBuilder.Instance.Labels = nil
			obj, _ := serviceBuilder.Build()
			service = obj.(*corev1.Service)
			err := serviceBuilder.Update(service)
			Expect(err).NotTo(HaveOccurred())

			// Expect to have a set of labels that are always included
			for label := range service.Labels {
				Expect(strings.HasPrefix(label, "app.kubernetes.io")).To(BeTrue())
			}
		})
	})
})
