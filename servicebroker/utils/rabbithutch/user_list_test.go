package rabbithutch_test

import (
	"errors"

	rabbithole "github.com/michaelklishin/rabbit-hole"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "servicebroker/utils/rabbithutch"
	"servicebroker/utils/rabbithutch/fakes"
)

var _ = Describe("Deleting the user", func() {
	var (
		rabbitClient *fakes.FakeAPIClient
		rabbithutch  RabbitHutch
	)

	BeforeEach(func() {
		rabbitClient = new(fakes.FakeAPIClient)
		rabbithutch = New(rabbitClient)
	})

	It("list all users", func() {
		userList := []rabbithole.UserInfo{
			rabbithole.UserInfo{
				Name: "fake-user-1",
			},
			rabbithole.UserInfo{
				Name: "fake-user-2",
			},
		}
		rabbitClient.ListUsersReturns(userList, nil)

		users, err := rabbithutch.UserList()

		Expect(err).NotTo(HaveOccurred())
		Expect(users).To(Equal([]string{"fake-user-1", "fake-user-2"}))
		Expect(rabbitClient.ListUsersCallCount()).To(Equal(1))
	})

	It("returns an error if it cannot list users", func() {
		rabbitClient.ListUsersReturns(nil, errors.New("fake list error"))

		users, err := rabbithutch.UserList()

		Expect(err).To(MatchError("fake list error"))
		Expect(users).To(BeNil())
	})
})
