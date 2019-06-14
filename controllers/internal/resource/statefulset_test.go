package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/controllers/internal/resource"
)

var _ = Describe("Resource", func() {
	Context("StatefulSet", func() {
		var instance rabbitmqv1beta1.RabbitmqCluster

		It("Creates a working StatefulSet with minimum requirements", func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{}
			instance.Namespace = "foo"
			instance.Name = "foo"

			sts := resource.GenerateStatefulSet(instance)

			requiredContainerPorts := []int32{5672, 15672}
			var actualContainerPorts []int32

			for _, container := range sts.Spec.Template.Spec.Containers {
				if container.Name == "rabbitmq" {
					for _, port := range container.Ports {
						actualContainerPorts = append(actualContainerPorts, port.ContainerPort)
					}
					break
				}
			}

			for _, port := range requiredContainerPorts {
				Expect(actualContainerPorts).To(ContainElement(port))
			}
		})
	})
})
