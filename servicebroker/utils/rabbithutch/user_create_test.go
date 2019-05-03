package rabbithutch_test

import (
	"errors"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"

	. "servicebroker/utils/rabbithutch"
	"servicebroker/utils/rabbithutch/fakes"
)

var _ = Describe("Creating Users", func() {
	var (
		rabbitClient *fakes.FakeAPIClient
		rabbithutch  RabbitHutch
	)

	BeforeEach(func() {
		rabbitClient = new(fakes.FakeAPIClient)
		rabbithutch = New(rabbitClient)
	})

	It("creates a user", func() {
		rabbitClient.UpdatePermissionsInReturns(&http.Response{StatusCode: http.StatusOK}, nil)

		password, err := rabbithutch.CreateUserAndGrantPermissions("fake-user", "fake-vhost", "")

		Expect(err).NotTo(HaveOccurred())
		Expect(rabbitClient.PutUserCallCount()).To(Equal(1))
		username, info := rabbitClient.PutUserArgsForCall(0)
		Expect(username).To(Equal("fake-user"))
		Expect(info.Tags).To(Equal("policymaker,management"))
		Expect(info.Password).To(MatchRegexp(`[a-zA-Z0-9\-_]{24}`))
		Expect(password).To(Equal(info.Password))
	})

	It("fails with an error if it cannot create a user", func() {
		rabbitClient.PutUserReturns(nil, errors.New("foo"))

		_, err := rabbithutch.CreateUserAndGrantPermissions("fake-user", "fake-vhost", "")

		Expect(err).To(MatchError("foo"))
	})

	It("fails with an error if the user already exists", func() {
		rabbitClient.PutUserReturns(&http.Response{StatusCode: http.StatusNoContent}, nil)

		_, err := rabbithutch.CreateUserAndGrantPermissions("fake-user", "fake-vhost", "")

		Expect(err).To(MatchError(brokerapi.ErrBindingAlreadyExists))
	})

	It("deletes the user if setting permissions fails", func() {
		rabbitClient.PutUserReturns(&http.Response{StatusCode: http.StatusOK}, nil)
		rabbitClient.UpdatePermissionsInReturns(nil, errors.New("cannot update permissions"))

		_, err := rabbithutch.CreateUserAndGrantPermissions("fake-user", "fake-vhost", "")

		Expect(err).To(MatchError("cannot update permissions"))
		Expect(rabbitClient.DeleteUserCallCount()).To(Equal(1))
		user := rabbitClient.DeleteUserArgsForCall(0)
		Expect(user).To(Equal("fake-user"))
	})

	It("grants the user full permissions to the vhost", func() {
		rabbitClient.UpdatePermissionsInReturns(&http.Response{StatusCode: http.StatusOK}, nil)

		_, err := rabbithutch.CreateUserAndGrantPermissions("fake-user", "fake-vhost", "")
		vhost, username, permissions := rabbitClient.UpdatePermissionsInArgsForCall(0)

		Expect(err).NotTo(HaveOccurred())

		Expect(rabbitClient.UpdatePermissionsInCallCount()).To(Equal(1))
		Expect(vhost).To(Equal("fake-vhost"))
		Expect(username).To(Equal("fake-user"))
		Expect(permissions.Configure).To(Equal(".*"))
		Expect(permissions.Read).To(Equal(".*"))
		Expect(permissions.Write).To(Equal(".*"))
	})

	When("user tags are specified", func() {
		It("creates a user with the tags", func() {
			rabbitClient.UpdatePermissionsInReturns(&http.Response{StatusCode: http.StatusOK}, nil)
			_, err := rabbithutch.CreateUserAndGrantPermissions("fake-user", "fake-vhost", "some-tags")
			username, info := rabbitClient.PutUserArgsForCall(0)

			Expect(err).NotTo(HaveOccurred())

			Expect(rabbitClient.PutUserCallCount()).To(Equal(1))
			Expect(username).To(Equal("fake-user"))
			Expect(info.Password).To(MatchRegexp(`[a-zA-Z0-9\-_]{24}`))
			Expect(info.Tags).To(Equal("some-tags"))
		})
	})
})
