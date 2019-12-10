package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
)

var _ = Context("IngressServices", func() {
	var (
		instance   rabbitmqv1beta1.RabbitmqCluster
		rmqBuilder resource.RabbitmqResourceBuilder
		scheme     *runtime.Scheme
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
		Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())
		instance = rabbitmqv1beta1.RabbitmqCluster{}
		instance.Namespace = "foo"
		instance.Name = "foo"
		rmqBuilder = resource.RabbitmqResourceBuilder{
			Instance: &instance,
			DefaultConfiguration: resource.DefaultConfiguration{
				Scheme: scheme,
			},
		}
	})

	It("generates Ingress Service with defaults", func() {
		serviceBuilder := rmqBuilder.IngressService()
		obj, err := serviceBuilder.Build()
		Expect(err).NotTo(HaveOccurred())
		service := obj.(*corev1.Service)

		By("generates a service object with the correct name and labels", func() {
			expectedName := instance.ChildResourceName("ingress")
			Expect(service.Name).To(Equal(expectedName))
			labels := service.Labels
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
			Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
		})

		By("generates a service object with the correct namespace", func() {
			Expect(service.Namespace).To(Equal(instance.Namespace))
		})

		By("generates a ClusterIP type service by default", func() {
			Expect(service.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
		})

		By("generates a service object with the correct selector", func() {
			Expect(service.Spec.Selector["app.kubernetes.io/name"]).To(Equal(instance.Name))
		})

		By("generates a service object with the correct ports exposed", func() {
			amqpPort := corev1.ServicePort{
				Name:     "amqp",
				Port:     5672,
				Protocol: corev1.ProtocolTCP,
			}
			httpPort := corev1.ServicePort{
				Name:     "http",
				Port:     15672,
				Protocol: corev1.ProtocolTCP,
			}
			prometheusPort := corev1.ServicePort{
				Name:     "prometheus",
				Port:     15692,
				Protocol: corev1.ProtocolTCP,
			}
			Expect(service.Spec.Ports).Should(ConsistOf(amqpPort, httpPort, prometheusPort))
		})

		By("generates the service without any annotation", func() {
			Expect(service.ObjectMeta.Annotations).To(BeNil())
		})

		By("setting the ownerreference", func() {
			Expect(service.ObjectMeta.OwnerReferences[0].Name).To(Equal("foo"))
		})
	})

	When("service type is specified in the RabbitmqCluster spec", func() {
		It("generates a service object of type LoadBalancer", func() {
			loadBalancerInstance := &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: v1.ObjectMeta{
					Name:      "name",
					Namespace: "mynamespace",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Service: rabbitmqv1beta1.RabbitmqClusterServiceSpec{
						Type: "LoadBalancer",
					},
				},
			}
			rmqBuilder.Instance = loadBalancerInstance
			serviceBuilder := rmqBuilder.IngressService()
			obj, err := serviceBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			loadBalancerService := obj.(*corev1.Service)
			Expect(err).NotTo(HaveOccurred())
			Expect(loadBalancerService.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
		})

		It("generates a service object of type ClusterIP", func() {
			clusterIPInstance := &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: v1.ObjectMeta{
					Name:      "name",
					Namespace: "mynamespace",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Service: rabbitmqv1beta1.RabbitmqClusterServiceSpec{
						Type: "ClusterIP",
					},
				},
			}
			rmqBuilder.Instance = clusterIPInstance
			serviceBuilder := rmqBuilder.IngressService()
			obj, err := serviceBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			clusterIPService := obj.(*corev1.Service)
			Expect(clusterIPService.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
		})

		It("generates a service object of type NodePort", func() {
			nodePortInstance := &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: v1.ObjectMeta{
					Name:      "name",
					Namespace: "mynamespace",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Service: rabbitmqv1beta1.RabbitmqClusterServiceSpec{
						Type: "NodePort",
					},
				},
			}
			rmqBuilder.Instance = nodePortInstance
			serviceBuilder := rmqBuilder.IngressService()
			obj, err := serviceBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			nodePortService := obj.(*corev1.Service)
			Expect(err).NotTo(HaveOccurred())
			Expect(nodePortService.Spec.Type).To(Equal(corev1.ServiceTypeNodePort))
		})
	})

	When("service type is specified on the rmqBuilder struct", func() {
		It("generates the service type as specified", func() {
			instance := &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: v1.ObjectMeta{
					Name:      "name",
					Namespace: "mynamespace",
				},
			}
			rmqBuilder.Instance = instance
			rmqBuilder.DefaultConfiguration.ServiceType = "NodePort"
			serviceBuilder := rmqBuilder.IngressService()
			obj, err := serviceBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			nodePortService := obj.(*corev1.Service)
			Expect(err).NotTo(HaveOccurred())
			Expect(nodePortService.Spec.Type).To(Equal(corev1.ServiceTypeNodePort))

		})

		It("generates the service type specified in the RabbitmqCluster spec", func() {
			loadBalancerInstance := &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: v1.ObjectMeta{
					Name:      "name",
					Namespace: "mynamespace",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Service: rabbitmqv1beta1.RabbitmqClusterServiceSpec{
						Type: "LoadBalancer",
					},
				},
			}
			rmqBuilder.Instance = loadBalancerInstance
			rmqBuilder.DefaultConfiguration.ServiceType = "ClusterIP"
			serviceBuilder := rmqBuilder.IngressService()
			obj, err := serviceBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			loadBalancerService := obj.(*corev1.Service)
			Expect(err).NotTo(HaveOccurred())
			Expect(loadBalancerService.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
		})
	})

	When("service annotations is specified in RabbitmqCluster spec", func() {
		It("generates a service object with the annotations as specified", func() {
			annotations := map[string]string{"service_annotation_a": "0.0.0.0/0"}
			instance := &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: v1.ObjectMeta{
					Name:      "name",
					Namespace: "mynamespace",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Service: rabbitmqv1beta1.RabbitmqClusterServiceSpec{
						Annotations: annotations,
					},
				},
			}
			rmqBuilder.Instance = instance
			serviceBuilder := rmqBuilder.IngressService()
			obj, err := serviceBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			service := obj.(*corev1.Service)
			Expect(err).NotTo(HaveOccurred())
			Expect(service.ObjectMeta.Annotations).To(Equal(annotations))
		})
	})

	When("service annotations are passed in as a function param and in RabbitmqCluster spec", func() {
		It("generates the service annotations as specified in the RabbitmqCluster spec", func() {
			expectedAnnotations := map[string]string{"service_annotation_a": "0.0.0.0/0"}
			ignoredAnnotations := map[string]string{"service_annotation_b": "0.0.0.0/1"}
			instance := &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: v1.ObjectMeta{
					Name:      "name",
					Namespace: "mynamespace",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Service: rabbitmqv1beta1.RabbitmqClusterServiceSpec{
						Annotations: expectedAnnotations,
					},
				},
			}
			rmqBuilder.Instance = instance
			rmqBuilder.DefaultConfiguration.ServiceAnnotations = ignoredAnnotations
			rmqBuilder.DefaultConfiguration.ServiceType = "NodePort"
			serviceBuilder := rmqBuilder.IngressService()
			obj, err := serviceBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			nodePortService := obj.(*corev1.Service)
			Expect(err).NotTo(HaveOccurred())
			Expect(nodePortService.ObjectMeta.Annotations).To(Equal(expectedAnnotations))
		})
	})

	When("service annotations are passed in when generating the service", func() {
		It("generates the service annotations as specified", func() {
			instance := &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: v1.ObjectMeta{
					Name:      "name",
					Namespace: "mynamespace",
				},
			}

			annotations := map[string]string{"service_annotation_a": "0.0.0.0/0"}
			rmqBuilder.Instance = instance
			rmqBuilder.DefaultConfiguration.ServiceAnnotations = annotations
			rmqBuilder.DefaultConfiguration.ServiceType = "NodePort"
			serviceBuilder := rmqBuilder.IngressService()
			obj, err := serviceBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			nodePortService := obj.(*corev1.Service)
			Expect(err).NotTo(HaveOccurred())
			Expect(nodePortService.ObjectMeta.Annotations).To(Equal(annotations))
		})
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
			serviceBuilder := rmqBuilder.IngressService()
			obj, err := serviceBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			ingressService := obj.(*corev1.Service)
			Expect(err).NotTo(HaveOccurred())
			testLabels(ingressService.Labels)
		})
	})

	Context("Update", func() {

		var (
			serviceBuilder *resource.IngressServiceBuilder
			ingressService *corev1.Service
			annotations    = map[string]string{"service_annotation_123": "0.0.0.0/0"}
		)

		Context("Annotations", func() {
			BeforeEach(func() {
				instance.Spec.Service.Annotations = annotations
				serviceBuilder = rmqBuilder.IngressService()
				ingressService = &corev1.Service{}
			})

			It("updates the service annotations", func() {
				Expect(serviceBuilder.Update(ingressService)).To(Succeed())
				Expect(ingressService.ObjectMeta.Annotations).To(Equal(annotations))
			})
		})

		Context("Labels", func() {
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

				ingressService = &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app.kubernetes.io/name":      instance.Name,
							"app.kubernetes.io/part-of":   "pivotal-rabbitmq",
							"this-was-the-previous-label": "should-be-deleted",
						},
					},
				}
				err := serviceBuilder.Update(ingressService)
				Expect(err).NotTo(HaveOccurred())
			})

			It("adds labels from the CR", func() {
				testLabels(ingressService.Labels)
			})

			It("restores the default labels", func() {
				labels := ingressService.Labels
				Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
				Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
				Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
			})

			It("deletes the labels that are removed from the CR", func() {
				Expect(ingressService.Labels).NotTo(HaveKey("this-was-the-previous-label"))
			})
		})
	})
})
