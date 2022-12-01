package internal_test

import (
	"crypto/sha512"
	"encoding/base64"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/topology"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("GenerateUserSettings", func() {
	var credentialSecret corev1.Secret
	var userTags []rabbitmqv1beta1.UserTag

	BeforeEach(func() {
		credentialSecret = corev1.Secret{
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"username": []byte("my-rabbit-user"),
				"password": []byte("a-secure-password"),
			},
		}
		userTags = []rabbitmqv1beta1.UserTag{"administrator", "monitoring"}
	})

	It("generates the expected rabbithole.UserSettings", func() {
		settings, err := internal.GenerateUserSettings(&credentialSecret, userTags)
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.Name).To(Equal("my-rabbit-user"))
		Expect(settings.Tags).To(ConsistOf("administrator", "monitoring"))
		Expect(settings.HashingAlgorithm.String()).To(Equal(rabbithole.HashingAlgorithmSHA512.String()))

		// The first 4 bytes of the PasswordHash will be the salt used in the hashing algorithm.
		// See https://www.rabbitmq.com/passwords.html#computing-password-hash.
		// We can take this salt and calculate what the correct hashed salted value would
		// be for our original plaintext password.
		passwordHashBytes, err := base64.StdEncoding.DecodeString(settings.PasswordHash)
		Expect(err).NotTo(HaveOccurred())

		salt := passwordHashBytes[0:4]
		saltedHash := sha512.Sum512([]byte(string(salt) + "a-secure-password"))
		Expect(base64.StdEncoding.EncodeToString([]byte(string(salt) + string(saltedHash[:])))).To(Equal(settings.PasswordHash))
	})
})
