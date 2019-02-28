package helpers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal/rabbitmq-for-kubernetes/pkg/helpers"
)

var _ = Describe("Kustomize", func() {
	Context("Build function", func() {
		It("parses the target yaml", func() {
			err := Build("../../templates", "anything", "anyNamespace")
			Expect(err).To(BeNil())
		})
	})
})
