package rabbithutch_test

import (
	"net/http"

	"servicebroker/utils/rabbithutch/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "servicebroker/utils/rabbithutch"
)

var _ = Describe("CreatePolicy", func() {

	var (
		name         string
		priority     int
		definition   map[string]interface{}
		rabbitClient *fakes.FakeAPIClient
		rabbithutch  RabbitHutch
	)

	BeforeEach(func() {
		rabbitClient = new(fakes.FakeAPIClient)
		rabbithutch = New(rabbitClient)
		name = "fake-policy-name"
		definition = map[string]interface{}{"fake-policy-key": "fake-policy-value"}
		priority = 42

	})

	It("creates a policy", func() {
		err := rabbithutch.CreatePolicy("my-service-instance-id", name, priority, definition)
		Expect(err).NotTo(HaveOccurred())

		Expect(rabbitClient.PutPolicyCallCount()).To(Equal(1))
		vhost, policyName, policy := rabbitClient.PutPolicyArgsForCall(0)
		Expect(vhost).To(Equal("my-service-instance-id"))
		Expect(policyName).To(Equal("fake-policy-name"))
		Expect(policy.Definition).To(BeEquivalentTo(map[string]interface{}{"fake-policy-key": "fake-policy-value"}))
		Expect(policy.Priority).To(Equal(42))
	})

	When("setting policies fails", func() {
		BeforeEach(func() {
			rabbitClient.PutPolicyReturns(&http.Response{StatusCode: http.StatusForbidden}, nil)
		})

		It("returns an error", func() {
			err := rabbithutch.CreatePolicy("my-service-instance-id", name, priority, definition)
			Expect(err).To(MatchError("http request failed with status code: 403"))
		})
	})
})
