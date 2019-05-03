package binding_test

import (
	"encoding/json"
	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"servicebroker/utils/binding"
)

var _ = Describe("Binding", func() {
	It("generates the right binding with TLS enabled", func() {
		b := binding.Builder{
			MgmtDomain:    "pivotal-rabbitmq.sys.philippinebrown.cf-app.com",
			Hostnames:     []string{"10.0.4.100", "10.0.4.101"},
			VHost:         "6418d19f-e9e8-4c8b-9c92-5087c89cbc46",
			Username:      "b2a5de47-796b-414d-bab4-eb299c268653",
			Password:      "cfrnvvjtr6t803ilrdhgbe8mn7",
			ProtocolPorts: fakeTLSProtocolPorts(),
			TLS:           true,
		}

		bindingMap, err := b.Build()
		Expect(err).NotTo(HaveOccurred())

		creds, err := json.Marshal(bindingMap)
		Expect(err).NotTo(HaveOccurred())

		expected, err := ioutil.ReadFile("fixtures/binding_tls.json")
		Expect(err).NotTo(HaveOccurred())

		Expect(string(creds)).To(MatchJSON(expected))
	})

	It("generates the right binding with TLS disabled", func() {
		b := binding.Builder{
			MgmtDomain:    "pivotal-rabbitmq.sys.philippinebrown.cf-app.com",
			Hostnames:     []string{"10.0.4.100", "10.0.4.101"},
			VHost:         "564da46b-e087-479f-ab63-5cf2aae0d098",
			Username:      "ce294862-03cc-4f4e-a6a8-69cb36e4f96e",
			Password:      "2143rkcobrqhdej1ke5ds5p8u6",
			TLS:           false,
			ProtocolPorts: fakeNonTLSProtocolPorts(),
		}

		bindingMap, err := b.Build()
		Expect(err).NotTo(HaveOccurred())

		creds, err := json.Marshal(bindingMap)
		Expect(err).NotTo(HaveOccurred())

		expected, err := ioutil.ReadFile("fixtures/binding_no_tls.json")
		Expect(err).NotTo(HaveOccurred())

		Expect(string(creds)).To(MatchJSON(expected))
	})
})

func fakeNonTLSProtocolPorts() map[string]int {
	return map[string]int{
		"amqp":           5672,
		"clustering":     25672,
		"http":           15672,
		"mqtt":           1883,
		"stomp":          61613,
		"http/web-stomp": 15674,
		"http/web-mqtt":  15675,
	}
}

func fakeTLSProtocolPorts() map[string]int {
	return map[string]int{
		"amqp/ssl":       5671,
		"clustering":     25672,
		"http":           15672,
		"mqtt":           1883,
		"mqtt/ssl":       8883,
		"stomp":          61613,
		"stomp/ssl":      61614,
		"http/web-stomp": 15674,
		"http/web-mqtt":  15675,
	}
}
