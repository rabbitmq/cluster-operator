package resource_test

import (
	b64 "encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Secret", func() {
	var instance rabbitmqv1beta1.RabbitmqCluster
	var secret *corev1.Secret
	var err error

	BeforeEach(func() {
		instance = rabbitmqv1beta1.RabbitmqCluster{}
		instance.Namespace = "foo"
		instance.Name = "foo"
	})

	Context("when succeeds", func() {
		BeforeEach(func() {
			secret, err = resource.GenerateSecret(instance)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates the secret 'rabbitmq-secret'", func() {
			expectedName := instance.Name + SecretSuffix
			Expect(secret.Name).To(Equal(expectedName))
		})

		It("creates the required labels", func() {
			Expect(secret.Labels["app"]).To(Equal("pivotal-rabbitmq"))
			Expect(secret.Labels["RabbitmqCluster"]).To(Equal(instance.Name))
		})

		It("creates a 'opaque' secret ", func() {
			Expect(secret.Type).To(Equal(corev1.SecretTypeOpaque))
		})

		It("creates a rabbitmq username that is base64 encoded and 24 characters in length", func() {
			username, ok := secret.Data["rabbitmq-username"]
			Expect(ok).NotTo(BeFalse())
			decodedUsername, err := b64.URLEncoding.DecodeString(string(username))
			Expect(err).NotTo(HaveOccurred())
			Expect(len(decodedUsername)).To(Equal(24))

		})

		It("creates a rabbitmq password that is base64 encoded and 24 characters in length", func() {
			password, ok := secret.Data["rabbitmq-password"]
			Expect(ok).NotTo(BeFalse())
			decodedPassword, err := b64.URLEncoding.DecodeString(string(password))
			Expect(err).NotTo(HaveOccurred())
			Expect(len(decodedPassword)).To(Equal(24))

		})
	})
})
