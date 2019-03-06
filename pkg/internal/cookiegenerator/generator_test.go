package cookiegenerator_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/cookiegenerator"
)

var _ = Describe("CookieGenerator", func() {
	Describe("Generate", func() {
		It("generates different passwords each time", func() {
			first, err := Generate()
			Expect(err).NotTo(HaveOccurred())

			second, err2 := Generate()
			Expect(err2).NotTo(HaveOccurred())

			Expect(first).NotTo(Equal(second))
		})

		It("generates url safe passwords", func() {
			randomStr, err := Generate()
			Expect(err).NotTo(HaveOccurred())
			Expect(randomStr).To(MatchRegexp("^[a-zA-Z0-9\\-_]{24}$"))
		})
	})
})
