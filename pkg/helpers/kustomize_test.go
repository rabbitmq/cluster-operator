package helpers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal/rabbitmq-for-kubernetes/pkg/helpers"
)

var _ = Describe("Kustomize", func() {
	Context("Build function", func() {
		It("parses the target yaml", func() {
			_, err := Build("../../templates", "anything", "anyNamespace")
			Expect(err).To(BeNil())
		})
	})

	Context("Decode", func() {
		It("parses yaml into K8s resources", func() {
			resources, err := Build("../../templates", "anything", "anyNamespace")
			objects, err := Decode(resources)

			Expect(err).To(BeNil())
			Expect(len(objects)).To(Equal(8))
		})
	})
})
