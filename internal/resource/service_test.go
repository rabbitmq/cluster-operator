package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Context("Services", func() {
	var (
		instance rabbitmqv1beta1.RabbitmqCluster
		service  *corev1.Service
	)

	BeforeEach(func() {
		instance = rabbitmqv1beta1.RabbitmqCluster{}
		instance.Namespace = "foo"
		instance.Name = "foo"
	})

	Describe("GenerateIngressService", func() {
		When("using generating Ingress Service with defaults", func() {
			BeforeEach(func() {
				service = resource.GenerateIngressService(instance, "", nil)
			})

			It("generates a service object with the correct name and labels", func() {
				expectedName := instance.ChildResourceName("ingress")
				Expect(service.Name).To(Equal(expectedName))
				Expect(service.ObjectMeta.Labels["app"]).To(Equal(instance.Name))
			})

			It("generates a service object with the correct namespace", func() {
				Expect(service.Namespace).To(Equal(instance.Namespace))
			})

			It("generates a ClusterIP type service by default", func() {
				Expect(service.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
			})

			It("generates a service object with the correct selector", func() {
				Expect(service.Spec.Selector["app"]).To(Equal(instance.Name))
			})

			It("generates a service object with the correct ports exposed", func() {
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

			It("generates the service without any annotation", func() {
				Expect(service.ObjectMeta.Annotations).To(BeNil())
			})
		})

		When("service type is specified in the RabbitmqCluster spec", func() {
			It("generates a service object of type LoadBalancer", func() {
				loadBalancerInstance := rabbitmqv1beta1.RabbitmqCluster{
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
				loadBalancerService := resource.GenerateIngressService(loadBalancerInstance, "", nil)
				Expect(loadBalancerService.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
			})

			It("generates a service object of type ClusterIP", func() {
				clusterIPInstance := rabbitmqv1beta1.RabbitmqCluster{
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
				clusterIPService := resource.GenerateIngressService(clusterIPInstance, "", nil)
				Expect(clusterIPService.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
			})

			It("generates a service object of type NodePort", func() {
				nodePortInstance := rabbitmqv1beta1.RabbitmqCluster{
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
				nodePortService := resource.GenerateIngressService(nodePortInstance, "", nil)
				Expect(nodePortService.Spec.Type).To(Equal(corev1.ServiceTypeNodePort))
			})
		})

		When("service type is specified through the function param", func() {
			It("generates the service type as specified", func() {
				instance := rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: v1.ObjectMeta{
						Name:      "name",
						Namespace: "mynamespace",
					},
				}
				nodePortService := resource.GenerateIngressService(instance, "NodePort", nil)
				Expect(nodePortService.Spec.Type).To(Equal(corev1.ServiceTypeNodePort))

			})

			It("generates the service type specified in the RabbitmqCluster spec", func() {
				loadBalancerInstance := rabbitmqv1beta1.RabbitmqCluster{
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
				loadBalancerService := resource.GenerateIngressService(loadBalancerInstance, "ClusterIP", nil)
				Expect(loadBalancerService.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
			})
		})

		When("service annotations is specified in RabbitmqCluster spec", func() {
			It("generates a service object with the annotations as specified", func() {
				annotations := map[string]string{"service_annotation_a": "0.0.0.0/0"}
				instance := rabbitmqv1beta1.RabbitmqCluster{
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
				service := resource.GenerateIngressService(instance, "", nil)
				Expect(service.ObjectMeta.Annotations).To(Equal(annotations))
			})
		})

		When("service annotations are passed in as a function param and in RabbitmqCluster spec", func() {
			It("generates the service annotations as specified in the RabbitmqCluster spec", func() {
				expectedAnnotations := map[string]string{"service_annotation_a": "0.0.0.0/0"}
				ignoredAnnotations := map[string]string{"service_annotation_b": "0.0.0.0/1"}
				instance := rabbitmqv1beta1.RabbitmqCluster{
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
				nodePortService := resource.GenerateIngressService(instance, "NodePort", ignoredAnnotations)
				Expect(nodePortService.ObjectMeta.Annotations).To(Equal(expectedAnnotations))
			})
		})

		When("service annotations are passed in when generating the service", func() {
			It("generates the service annotations as specified", func() {
				instance := rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: v1.ObjectMeta{
						Name:      "name",
						Namespace: "mynamespace",
					},
				}
				annotations := map[string]string{"service_annotation_a": "0.0.0.0/0"}
				nodePortService := resource.GenerateIngressService(instance, "NodePort", annotations)
				Expect(nodePortService.ObjectMeta.Annotations).To(Equal(annotations))
			})
		})
	})

	Describe("GenerateHeadlessService", func() {
		BeforeEach(func() {
			service = resource.GenerateHeadlessService(instance)
		})

		It("generates a service object with the correct name", func() {
			Expect(service.Name).To(Equal(instance.ChildResourceName("headless")))
		})

		It("generates a service object with the correct namespace", func() {
			Expect(service.Namespace).To(Equal(instance.Namespace))
		})

		It("generates a service object with the correct label", func() {
			Expect(service.ObjectMeta.Labels["app"]).To(Equal(instance.Name))
		})

		It("generates a service object with the correct selector", func() {
			Expect(service.Spec.Selector["app"]).To(Equal(instance.Name))
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
})
