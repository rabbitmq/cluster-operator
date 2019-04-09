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
				Name:        "some-marketplace-name",
				ID:          "some-id",
				Description: "some-description",
				Plans: []Plan{
					Plan{
						Name:        "some-dedicated-name",
						ID:          "some-dedicated-plan-id",
						Description: "I'm a dedicated plan",
					}},
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
