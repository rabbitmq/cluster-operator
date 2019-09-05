package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Service", func() {
	var instance rabbitmqv1beta1.RabbitmqCluster
	var service *corev1.Service

	BeforeEach(func() {
		instance = rabbitmqv1beta1.RabbitmqCluster{}
		instance.Namespace = "foo"
		instance.Name = "foo"
	})

	Context("succeeds", func() {

		BeforeEach(func() {
			service = resource.GenerateService(instance, "", nil)
		})

		It("creates a service object with the correct name and labels", func() {
			expectedName := instance.Name + "-rabbitmq-ingress"
			Expect(service.Name).To(Equal(expectedName))
			Expect(service.ObjectMeta.Labels["app"]).To(Equal(instance.Name))
		})

		It("creates a ClusterIP type service by default", func() {
			Expect(service.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
		})

		It("creates a service object with the correct selector", func() {
			Expect(service.Spec.Selector["app"]).To(Equal(instance.Name))
		})

		It("exposes the amqp, http, and prometheus ports", func() {
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

		It("creates a LoadBalancer type service when specified in the RabbitmqCluster spec", func() {
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
			loadBalancerService := resource.GenerateService(loadBalancerInstance, "", nil)
			Expect(loadBalancerService.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
		})

		It("creates a ClusterIP type service when specified in the RabbitmqCluster spec", func() {
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
			clusterIPService := resource.GenerateService(clusterIPInstance, "", nil)
			Expect(clusterIPService.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
		})

		It("creates a NodePort type service when specified in the RabbitmqCluster spec", func() {
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
			nodePortService := resource.GenerateService(nodePortInstance, "", nil)
			Expect(nodePortService.Spec.Type).To(Equal(corev1.ServiceTypeNodePort))
		})

		Context("when service type is specified through the function param", func() {
			It("creates the service type as specified", func() {
				instance := rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: v1.ObjectMeta{
						Name:      "name",
						Namespace: "mynamespace",
					},
				}
				nodePortService := resource.GenerateService(instance, "NodePort", nil)
				Expect(nodePortService.Spec.Type).To(Equal(corev1.ServiceTypeNodePort))

			})

			It("creates the service type specified in the RabbitmqCluster spec when both are present", func() {
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
				loadBalancerService := resource.GenerateService(loadBalancerInstance, "ClusterIP", nil)
				Expect(loadBalancerService.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
			})
		})

		When("service annotations is specified in RabbitmqCluster spec", func() {
			It("creates the service annotations as specified", func() {
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
				service := resource.GenerateService(instance, "", nil)
				Expect(service.ObjectMeta.Annotations).To(Equal(annotations))
			})
		})

		When("service annotations are passed in while generating the service and in RabbitmqCluster spec", func() {
			It("creates the service annotations as specified in the RabbitmqCluster spec", func() {
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
				nodePortService := resource.GenerateService(instance, "NodePort", ignoredAnnotations)
				Expect(nodePortService.ObjectMeta.Annotations).To(Equal(expectedAnnotations))
			})
		})

		When("service annotations are passed in while generating the service", func() {
			It("creates the service annotations as specified", func() {
				instance := rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: v1.ObjectMeta{
						Name:      "name",
						Namespace: "mynamespace",
					},
				}
				annotations := map[string]string{"service_annotation_a": "0.0.0.0/0"}
				nodePortService := resource.GenerateService(instance, "NodePort", annotations)
				Expect(nodePortService.ObjectMeta.Annotations).To(Equal(annotations))
			})
		})

		When("service annotations is not specified in RabbitmqCluster spec or not passed in", func() {
			It("creates the service without any annotation", func() {
				instance := rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: v1.ObjectMeta{
						Name:      "name",
						Namespace: "mynamespace",
					},
				}
				service := resource.GenerateService(instance, "", nil)
				Expect(service.ObjectMeta.Annotations).To(BeNil())
			})
		})
	})
})
