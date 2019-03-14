package generator_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/resourcegenerator"
)

var _ = Describe("Kustomize", func() {
	var generator ResourceGenerator

	BeforeEach(func() {
		generator = NewKustomizeResourceGenerator("./fixtures")
	})

	Context("Build function", func() {
		It("parses the target yaml into k8s resource", func() {
			generationContext := GenerationContext{
				InstanceName: "rabbit",
				Namespace:    "rabbit-namespace",
				Nodes:        int32(3),
			}
			objects, err := generator.Build(generationContext)

			Expect(err).To(BeNil())
			Expect(len(objects)).To(Equal(7))
		})
	})
})
