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
		instance       rabbitmqv1beta1.Cluster
		cluster        *resource.RabbitmqResourceBuilder
		serviceBuilder *resource.HeadlessServiceBuilder
		service        *corev1.Service
	)

	BeforeEach(func() {
		instance = rabbitmqv1beta1.Cluster{}
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
	})

	Context("Update with instance labels", func() {
		BeforeEach(func() {
			instance = rabbitmqv1beta1.Cluster{
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

			service = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":      instance.Name,
						"app.kubernetes.io/part-of":   "rabbitmq",
						"this-was-the-previous-label": "should-be-deleted",
					},
				},
			}
			err := serviceBuilder.Update(service)
			Expect(err).NotTo(HaveOccurred())
		})

		It("adds labels from the CR", func() {
			testLabels(service.Labels)
		})

		It("restores the default labels", func() {
			labels := service.Labels
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
			Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("rabbitmq"))
		})

		It("deletes the labels that are removed from the CR", func() {
			Expect(service.Labels).NotTo(HaveKey("this-was-the-previous-label"))
		})
	})

	Context("Update with instance annotations", func() {
		BeforeEach(func() {
			instance = rabbitmqv1beta1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rabbit-labelled",
				},
			}
			instance.Annotations = map[string]string{
				"my-annotation":               "i-like-this",
				"kubernetes.io/name":          "i-do-not-like-this",
				"kubectl.kubernetes.io/name":  "i-do-not-like-this",
				"k8s.io/name":                 "i-do-not-like-this",
				"kubernetes.io/other":         "i-do-not-like-this",
				"kubectl.kubernetes.io/other": "i-do-not-like-this",
				"k8s.io/other":                "i-do-not-like-this",
			}

			service = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"i-was-here-already":            "please-dont-delete-me",
						"im-here-to-stay.kubernetes.io": "for-a-while",
						"kubernetes.io/name":            "should-stay",
						"kubectl.kubernetes.io/name":    "should-stay",
						"k8s.io/name":                   "should-stay",
					},
				},
			}
			err := serviceBuilder.Update(service)
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates service annotations", func() {
			expectedAnnotations := map[string]string{
				"i-was-here-already":            "please-dont-delete-me",
				"im-here-to-stay.kubernetes.io": "for-a-while",
				"kubernetes.io/name":            "should-stay",
				"kubectl.kubernetes.io/name":    "should-stay",
				"k8s.io/name":                   "should-stay",
				"my-annotation":                 "i-like-this",
			}

			Expect(service.Annotations).To(Equal(expectedAnnotations))
		})
	})

	Context("Update Spec", func() {
		BeforeEach(func() {
			instance = rabbitmqv1beta1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rabbit-spec",
				},
			}

			service = &corev1.Service{
				Spec: corev1.ServiceSpec{
					ClusterIP: "1.2.3.4",
					Selector: map[string]string{
						"some-selector": "some-tag",
					},
					Ports: []corev1.ServicePort{
						{
							Protocol: corev1.ProtocolTCP,
							Port:     43691,
							Name:     "epmdf",
						},
					},
				},
			}
			err := serviceBuilder.Update(service)
			Expect(err).NotTo(HaveOccurred())
		})

		It("sets the required Spec", func() {
			expectedSpec := corev1.ServiceSpec{
				ClusterIP: "None",
				Selector: map[string]string{
					"app.kubernetes.io/name": "rabbit-spec",
				},
				Ports: []corev1.ServicePort{
					{
						Protocol: corev1.ProtocolTCP,
						Port:     4369,
						Name:     "epmd",
					},
				},
				PublishNotReadyAddresses: true,
			}

			Expect(service.Spec).To(Equal(expectedSpec))
		})
	})
})
