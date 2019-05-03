package rabbithutch_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "servicebroker/utils/rabbithutch"
	"servicebroker/utils/rabbithutch/fakes"

	rabbithole "github.com/michaelklishin/rabbit-hole"
)

var _ = Describe("VHostExists()", func() {
	var (
		rabbitClient *fakes.FakeAPIClient
		rabbithutch  RabbitHutch
	)

	BeforeEach(func() {
		rabbitClient = new(fakes.FakeAPIClient)
		rabbithutch = New(rabbitClient)
	})
	AfterEach(func() {
		Expect(rabbitClient.GetVhostArgsForCall(0)).To(Equal("fake-vhost"))
	})

	When("the vhost does not exist", func() {
		BeforeEach(func() {
			rabbitClient.GetVhostReturns(nil, rabbithole.ErrorResponse{StatusCode: http.StatusNotFound})
		})

		It("returns false", func() {
			ok, err := rabbithutch.VHostExists("fake-vhost")
			Expect(ok).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("we fail to query the vhost", func() {
		BeforeEach(func() {
			rabbitClient.GetVhostReturns(nil, rabbithole.ErrorResponse{StatusCode: http.StatusInternalServerError})
		})

		It("fails with an error saying the vhost could not be retrieved", func() {
			ok, err := rabbithutch.VHostExists("fake-vhost")
			Expect(err).To(MatchError(rabbithole.ErrorResponse{StatusCode: http.StatusInternalServerError}))
			Expect(ok).To(BeFalse())
		})
	})

	When("the vhost exists", func() {
		BeforeEach(func() {
			rabbitClient.GetVhostReturns(&rabbithole.VhostInfo{}, nil)
		})

		It("returns true", func() {
			ok, err := rabbithutch.VHostExists("fake-vhost")
			Expect(ok).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
