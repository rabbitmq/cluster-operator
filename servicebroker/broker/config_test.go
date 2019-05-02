package broker_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "servicebroker/broker"
)

var _ = Describe("Config", func() {
	It("reads a minimal config", func() {
		conf, err := ReadConfig(fixture("config.yml"))
		Expect(err).NotTo(HaveOccurred())
		Expect(conf).To(Equal(Config{
			Broker: Broker{
				Port:     8080,
				Username: "some-broker-user",
				Password: "some-broker-password",
			},
			ServiceCatalog: ServiceCatalog{
				ID:          "00000000-0000-0000-0000-000000000000",
				Name:        "p-rabbitmq-k8s",
				Description: "RabbitMQ on K8s",
				Plans: []Plan{
					Plan{
						ID:          "11111111-1111-1111-1111-111111111111",
						Name:        "ha",
						Description: "HA RabbitMQ on K8s",
					},
					Plan{
						ID:          "22222222-2222-2222-2222-222222222222",
						Name:        "single",
						Description: "Single-node RabbitMQ on K8s",
					},
				},
			},
			RabbitMQ: RabbitMQ{
				DNSHost:          "",
				Administrator:    AdminCredentials{Username: "admin", Password: "pass"},
				Management:       ManagementCredentials{Username: "admin"},
				ManagementDomain: "pivotal-rabbitmq.127.0.0.1",
				RegularUserTags:  "policymaker,management",
				TLS:              false,
			},
		}))
	})

	It("fails if no broker configuration is provided", func() {
		_, err := ReadConfig(fixture("config_without_broker.yml"))
		Expect(err).To(MatchError("Config file has missing fields: broker.port, broker.username, broker.password"))
	})

	It("fails if no plans are provided", func() {
		_, err := ReadConfig(fixture("config_without_plans.yml"))
		Expect(err).To(MatchError("Config file has missing fields: service_catalog.plans"))
	})

})

func fixture(name string) string {
	path, err := filepath.Abs(filepath.Join("fixtures", name))
	Expect(err).NotTo(HaveOccurred())
	return path
}
