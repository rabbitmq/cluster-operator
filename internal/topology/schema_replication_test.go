package internal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rabbitmq/cluster-operator/internal/topology"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("GenerateSchemaReplicationParameters", func() {
	var secret corev1.Secret

	BeforeEach(func() {
		secret = corev1.Secret{
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"username": []byte("a-random-user"),
				"password": []byte("a-random-password"),
			},
		}
	})
	When("endpoints are provided as function parameter", func() {
		It("generates expected replication parameters", func() {
			parameters, err := internal.GenerateSchemaReplicationParameters(&secret, "a.endpoints.local:5672,b.endpoints.local:5672,c.endpoints.local:5672")
			Expect(err).NotTo(HaveOccurred())
			Expect(parameters.Username).To(Equal("a-random-user"))
			Expect(parameters.Password).To(Equal("a-random-password"))
			Expect(parameters.Endpoints).To(Equal([]string{
				"a.endpoints.local:5672",
				"b.endpoints.local:5672",
				"c.endpoints.local:5672",
			}))
		})
	})

	When("endpoints are provided in the secret object and as function parameter", func() {
		It("endpoints parameter takes precedence over endpoints from secret object", func() {
			secret.Data["endpoints"] = []byte("some-url-value:1234")
			parameters, err := internal.GenerateSchemaReplicationParameters(&secret, "a.endpoints.local:5672,b.endpoints.local:5672,c.endpoints.local:5672")
			Expect(err).NotTo(HaveOccurred())
			Expect(parameters.Username).To(Equal("a-random-user"))
			Expect(parameters.Password).To(Equal("a-random-password"))
			Expect(parameters.Endpoints).To(Equal([]string{
				"a.endpoints.local:5672",
				"b.endpoints.local:5672",
				"c.endpoints.local:5672",
			}))
		})
	})

	When("endpoints are provided only in the secret object", func() {
		It("generates expected replication parameters", func() {
			secret.Data["endpoints"] = []byte("a.endpoints.local:5672,b.endpoints.local:5672,c.endpoints.local:5672")
			parameters, err := internal.GenerateSchemaReplicationParameters(&secret, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(parameters.Username).To(Equal("a-random-user"))
			Expect(parameters.Password).To(Equal("a-random-password"))
			Expect(parameters.Endpoints).To(Equal([]string{
				"a.endpoints.local:5672",
				"b.endpoints.local:5672",
				"c.endpoints.local:5672",
			}))
		})
	})
})
