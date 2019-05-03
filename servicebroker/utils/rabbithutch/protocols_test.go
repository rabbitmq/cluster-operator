package rabbithutch_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "servicebroker/utils/rabbithutch"
	"servicebroker/utils/rabbithutch/fakes"

	rabbithole "github.com/michaelklishin/rabbit-hole"
)

var _ = Describe("Binding a RMQ service instance", func() {
	var (
		rabbitClient *fakes.FakeAPIClient
		rabbithutch  RabbitHutch
	)

	BeforeEach(func() {
		rabbitClient = new(fakes.FakeAPIClient)
		rabbithutch = New(rabbitClient)
	})

	Describe("ProtocolPorts()", func() {
		It("reads the protocol ports", func() {
			rabbitClient.ProtocolPortsReturns(fakeProtocolPorts(), nil)

			protocolPorts, err := rabbithutch.ProtocolPorts()

			Expect(err).NotTo(HaveOccurred())
			Expect(protocolPorts).To(SatisfyAll(
				HaveKeyWithValue("amqp/ssl", 5671),
				HaveKeyWithValue("clustering", 25672),
			))
		})

		When("it cannot read the protocol ports", func() {
			BeforeEach(func() {
				rabbitClient.ProtocolPortsReturns(nil, fmt.Errorf("failed to read protocol ports"))
			})

			It("fails with an error", func() {
				protocolPorts, err := rabbithutch.ProtocolPorts()

				Expect(protocolPorts).To(BeNil())
				Expect(err).To(MatchError("failed to read protocol ports"))
			})
		})
	})
})

func fakeProtocolPorts() map[string]rabbithole.Port {
	return map[string]rabbithole.Port{
		"amqp/ssl":   5671,
		"clustering": 25672,
	}
}
