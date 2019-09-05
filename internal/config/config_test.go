package config_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/config"
)

var _ = Describe("NewConfig", func() {
	It("should return a valid config", func() {
		rawConfig := `
SERVICE:
  TYPE: test-type
  ANNOTATIONS:
    some-key: some-annotation
IMAGE_PULL_SECRET: thought-leader
IMAGE_REPOSITORY: some-great-repo
`
		config, err := config.NewConfig([]byte(rawConfig))
		Expect(err).NotTo(HaveOccurred())
		Expect(config.Service.Type).To(Equal("test-type"))
		Expect(config.Service.Annotations["some-key"]).To(Equal("some-annotation"))
		Expect(config.ImagePullSecret).To(Equal("thought-leader"))
		Expect(config.ImageRepository).To(Equal("some-great-repo"))
	})

	It("should return an error if it fails to unmarshal", func() {
		rawConfig := `iamnotavalidyamlfile`
		_, err := config.NewConfig([]byte(rawConfig))
		Expect(err).To(MatchError(ContainSubstring("could not unmarshal config")))
	})

	It("should return an empty Config if the file is empty", func() {
		rawConfig := ``
		_, err := config.NewConfig([]byte(rawConfig))
		Expect(err).NotTo(HaveOccurred())
	})

	Context("optional config parameters", func() {
		When("'SERVICE.TYPE' is missing", func() {
			It("returns a valid Config", func() {
				rawConfig := `
SERVICE:
  ANNOTATIONS:
    some-key: some-annotation
IMAGE_PULL_SECRET: thought-leader
IMAGE_REPOSITORY: some-great-repo
`
				config, err := config.NewConfig([]byte(rawConfig))
				Expect(err).NotTo(HaveOccurred())
				Expect(config.Service.Annotations["some-key"]).To(Equal("some-annotation"))
				Expect(config.Service.Type).To(Equal(""))
			})
		})
		When("'SERVICE.ANNOTATIONS' is missing", func() {
			It("returns a valid Config", func() {
				rawConfig := `
SERVICE:
  TYPE: test-type
IMAGE_PULL_SECRET: thought-leader
IMAGE_REPOSITORY: some-great-repo
`
				config, err := config.NewConfig([]byte(rawConfig))
				Expect(err).NotTo(HaveOccurred())
				Expect(config.Service.Type).To(Equal("test-type"))
				Expect(config.Service.Annotations).To(BeNil())
			})
		})

		When("'IMAGE_PULL_SECRET' is missing", func() {
			It("returns a valid Config", func() {
				rawConfig := `
SERVICE:
  TYPE: test-type
  ANNOTATIONS:
    some-key: some-annotation
IMAGE_REPOSITORY: some-great-repo
`
				config, err := config.NewConfig([]byte(rawConfig))
				Expect(err).NotTo(HaveOccurred())
				Expect(config.ImageRepository).To(Equal("some-great-repo"))
				Expect(config.ImagePullSecret).To(Equal(""))
			})
		})
	})
})
