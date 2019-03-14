package plans_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/plans"
)

var _ = Describe("Plans", func() {
	var plans = New()

	Describe("Get", func() {
		Context("existing plans", func() {
			It("single plan has 1 node", func() {
				plan, resultErr := plans.Get("single")

				Expect(resultErr).To(BeNil())
				Expect(plan.Nodes).To(Equal(int32(1)))
			})
			It("ha plan has 3 nodes", func() {
				plan, resultErr := plans.Get("ha")

				Expect(resultErr).To(BeNil())
				Expect(plan.Nodes).To(Equal(int32(3)))
			})
		})
		Context("unrecognised plans", func() {
			It("unkown plan name", func() {
				_, resultErr := plans.Get("no-message-loss")

				Expect(resultErr).To(Equal(UnrecognisedPlanError))
			})
			It("empty string", func() {
				_, resultErr := plans.Get("")

				Expect(resultErr).To(Equal(UnrecognisedPlanError))
			})
		})
	})
})
