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
	var serviceName string

	BeforeEach(func() {
		instance = rabbitmqv1beta1.RabbitmqCluster{}
		instance.Namespace = "foo"
		instance.Name = "foo"
		serviceName = "p-" + instance.Name
	})

	Context("succeeds", func() {

		BeforeEach(func() {
			service = resource.GenerateService(instance)
		})

		It("creates a service object with the correct name and labels", func() {
			Expect(service.Name).To(Equal(serviceName))
			Expect(service.ObjectMeta.Labels["app"]).To(Equal(instance.Name))
		})

		It("creates a ClusterIP type service", func() {
			Expect(service.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
		})

		It("creates a service object with the correct selector", func() {
			Expect(service.Spec.Selector["app"]).To(Equal(instance.Name))
		})

		It("exposes the amqp and http ports", func() {
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
			Expect(service.Spec.Ports).Should(ConsistOf(amqpPort, httpPort))
		})

		It("creates a LoadBalancer type service when specified", func() {
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
			loadBalancerService := resource.GenerateService(loadBalancerInstance)
			Expect(loadBalancerService.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
		})

		It("creates a ClusterIP type service when specified", func() {
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
			clusterIPService := resource.GenerateService(clusterIPInstance)
			Expect(clusterIPService.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
		})

		It("creates a NodePort type service when specified", func() {
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
			nodePortService := resource.GenerateService(nodePortInstance)
			Expect(nodePortService.Spec.Type).To(Equal(corev1.ServiceTypeNodePort))
		})
	})
})
