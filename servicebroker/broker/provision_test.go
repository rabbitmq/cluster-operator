package broker_test

import (
	broke "servicebroker/broker"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
)

var _ = Describe("Provision", func() {

	var (
		instanceID = "rmq-k8s-instance"
		broker     broke.RabbitMQServiceBroker
	)

	BeforeEach(func() {
		broker = defaultServiceBroker().(broke.RabbitMQServiceBroker)
	})

	It("returns a yaml spec for a single plan", func() {

		details := brokerapi.ProvisionDetails{
			PlanID: "22222222-2222-2222-2222-222222222222",
		}

		spec, err := broker.GenerateSpec(instanceID, details)
		Expect(err).NotTo(HaveOccurred())
		Expect(spec.Spec.Plan).To(Equal("single"))

	})

	It("returns a yaml spec for a ha plan", func() {

		details := brokerapi.ProvisionDetails{
			PlanID: "11111111-1111-1111-1111-111111111111",
		}

		spec, err := broker.GenerateSpec(instanceID, details)
		Expect(err).NotTo(HaveOccurred())
		Expect(spec.Spec.Plan).To(Equal("ha"))

	})
	It("returns an error for a non-configured plan", func() {

		details := brokerapi.ProvisionDetails{
			PlanID: "00000000-0000-0000-0000-000000000000",
		}

		_, err := broker.GenerateSpec(instanceID, details)
		Expect(err).To(MatchError("Unknown plan ID 00000000-0000-0000-0000-000000000000"))

	})

	It("creates a spec in the `rabbitmq-for-kubernetes` namespace", func() {

		details := brokerapi.ProvisionDetails{
			PlanID: "22222222-2222-2222-2222-222222222222",
		}

		spec, err := broker.GenerateSpec(instanceID, details)
		Expect(err).NotTo(HaveOccurred())
		Expect(spec.ObjectMeta.Namespace).To(Equal("rabbitmq-for-kubernetes"))
	})

	It("creates a spec with a name based on the `instanceID`", func() {

		details := brokerapi.ProvisionDetails{
			PlanID: "22222222-2222-2222-2222-222222222222",
		}

		spec, err := broker.GenerateSpec(instanceID, details)
		Expect(err).NotTo(HaveOccurred())
		Expect(spec.ObjectMeta.Name).To(Equal("rmq-k8s-instance"))
	})

})
