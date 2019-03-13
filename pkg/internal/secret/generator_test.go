package secret_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	resourcegenerator "github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/resourcegenerator"
	"github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/secret"
)

var _ = Describe("SecretGenerator", func() {
	var generationContext = resourcegenerator.GenerationContext{
		InstanceName: "test",
		Namespace:    "test-namespace",
		Nodes:        int32(1),
	}
	Describe("Generate", func() {
		It("generates different passwords each time", func() {

			first, err := secret.New(generationContext)
			Expect(err).NotTo(HaveOccurred())

			second, err2 := secret.New(generationContext)
			Expect(err2).NotTo(HaveOccurred())

			Expect(first.Data["erlang-cookie"]).NotTo(Equal(second.Data["erlang-cookie"]))
		})

		It("generates url safe passwords", func() {
			secret, err := secret.New(generationContext)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(secret.Data["erlang-cookie"])).To(MatchRegexp("^[a-zA-Z0-9\\-_]{24}$"))
		})
	})
})
