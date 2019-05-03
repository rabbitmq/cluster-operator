package rabbithutch_test

import (
	"net/http"

	"servicebroker/utils/rabbithutch/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "servicebroker/utils/rabbithutch"
)

var _ = Describe("UserPermissions", func() {

	var (
		rabbitClient *fakes.FakeAPIClient
		rabbithutch  RabbitHutch
	)

	BeforeEach(func() {
		rabbitClient = new(fakes.FakeAPIClient)
		rabbithutch = New(rabbitClient)
	})
	It("grants permissions on the vhost  user", func() {
		rabbitClient.UpdatePermissionsInReturns(&http.Response{StatusCode: http.StatusNoContent}, nil)
		err := rabbithutch.AssignPermissionsTo("fake-vhost", "fake-user")
		Expect(err).NotTo(HaveOccurred())

		Expect(rabbitClient.UpdatePermissionsInCallCount()).To(Equal(1))
		vhost, username, permissions := rabbitClient.UpdatePermissionsInArgsForCall(0)
		Expect(vhost).To(Equal("fake-vhost"))
		Expect(username).To(Equal("fake-user"))
		Expect(permissions.Configure).To(Equal(".*"))
		Expect(permissions.Read).To(Equal(".*"))
		Expect(permissions.Write).To(Equal(".*"))
	})

	When("granting permissions fails", func() {
		BeforeEach(func() {
			rabbitClient.UpdatePermissionsInReturns(&http.Response{StatusCode: http.StatusForbidden}, nil)
		})

		It("returns an error", func() {
			err := rabbithutch.AssignPermissionsTo("fake-vhost", "fake-user")
			Expect(err).To(MatchError("http request failed with status code: 403"))
		})
	})

})
