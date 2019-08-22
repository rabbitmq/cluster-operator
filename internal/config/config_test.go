package config_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/config"
)

var _ = Describe("NewServiceConfig", func() {
	It("should return a valid service config", func() {
		rawServiceConfig := `
TYPE: test-type
ANNOTATIONS:
  some-key: some-annotation
`
		serviceConfig, err := config.NewServiceConfig([]byte(rawServiceConfig))
		Expect(err).NotTo(HaveOccurred())
		Expect(serviceConfig.Type).To(Equal("test-type"))
		Expect(serviceConfig.Annotations["some-key"]).To(Equal("some-annotation"))
	})

	It("should return an error if it fails to unmarshal", func() {
		rawServiceConfig := `iamnotavalidyamlfile`
		_, err := config.NewServiceConfig([]byte(rawServiceConfig))
		Expect(err).To(MatchError(ContainSubstring("could not unmarshal config")))
	})

	It("should return an empty ServiceConfig if the file is empty", func() {
		rawServiceConfig := ``
		_, err := config.NewServiceConfig([]byte(rawServiceConfig))
		Expect(err).NotTo(HaveOccurred())
	})

	Context("optional service config parameters", func() {
		When("'TYPE' is missing", func() {
			It("returns a valid ServiceConfig", func() {
				rawServiceConfig := `
ANNOTATIONS:
  some-key: some-annotation
`
				serviceConfig, err := config.NewServiceConfig([]byte(rawServiceConfig))
				Expect(err).NotTo(HaveOccurred())
				Expect(serviceConfig.Annotations["some-key"]).To(Equal("some-annotation"))
				Expect(serviceConfig.Type).To(Equal(""))
			})
		})
		When("'ANNOTATIONS' is missing", func() {
			It("returns a valid ServiceConfig", func() {
				rawServiceConfig := `
TYPE: test-type
`
				serviceConfig, err := config.NewServiceConfig([]byte(rawServiceConfig))
				Expect(err).NotTo(HaveOccurred())
				Expect(serviceConfig.Type).To(Equal("test-type"))
				Expect(serviceConfig.Annotations).To(BeNil())
			})
		})
	})
})
