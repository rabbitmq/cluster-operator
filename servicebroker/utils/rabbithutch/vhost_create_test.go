package rabbithutch_test

import (
	"fmt"
	"net/http"
	"servicebroker/utils/rabbithutch/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "servicebroker/utils/rabbithutch"
)

var _ = Describe("VhostCreate", func() {
	var (
		rabbitClient *fakes.FakeAPIClient
		rabbithutch  RabbitHutch
	)

	BeforeEach(func() {
		rabbitClient = new(fakes.FakeAPIClient)
		rabbithutch = New(rabbitClient)
	})

	It("creates a vhost", func() {
		err := rabbithutch.VHostCreate("fake-vhost")
		Expect(err).NotTo(HaveOccurred())

		Expect(rabbitClient.PutVhostCallCount()).To(Equal(1))
		Expect(rabbitClient.PutVhostArgsForCall(0)).To(Equal("fake-vhost"))
	})

	When("the vhost creation fails", func() {
		It("returns an error when the RMQ API returns an error", func() {
			rabbitClient.PutVhostReturns(nil, fmt.Errorf("vhost-creation-failed"))

			err := rabbithutch.VHostCreate("fake-vhost")
			Expect(err).To(MatchError("vhost-creation-failed"))
		})

		It("returns an error when the RMQ API returns a bad HTTP response code", func() {
			rabbitClient.PutVhostReturns(&http.Response{StatusCode: http.StatusInternalServerError}, nil)

			err := rabbithutch.VHostCreate("fake-vhost")

			Expect(err).To(MatchError("http request failed with status code: 500"))
		})
	})
})
