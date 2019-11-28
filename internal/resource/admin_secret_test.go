package resource_test

import (
	b64 "encoding/base64"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("AdminSecret", func() {
	var (
		secret   *corev1.Secret
		instance = rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: v1.ObjectMeta{
				Name:      "a name",
				Namespace: "a namespace",
			},
		}

		rabbitmqCluster *resource.RabbitmqResourceBuilder
	)

	BeforeEach(func() {
		var err error

		rabbitmqCluster = &resource.RabbitmqResourceBuilder{
			Instance: &instance,
		}
		secret, err = rabbitmqCluster.AdminSecret()
		Expect(err).NotTo(HaveOccurred())
	})

	It("creates the secret with correct name and namespace", func() {
		Expect(secret.Name).To(Equal(instance.ChildResourceName("admin")))
		Expect(secret.Namespace).To(Equal("a namespace"))
	})

	It("creates the required labels", func() {
		labels := secret.Labels
		Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
		Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
		Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
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

	Context("label inheritance", func() {
		BeforeEach(func() {
			instance.Labels = map[string]string{
				"app.kubernetes.io/foo": "bar",
				"foo":                   "bar",
				"rabbitmq":              "is-great",
				"foo/app.kubernetes.io": "edgecase",
			}
		})

		It("has the labels from the CRD on the admin secret", func() {
			adminSecret, _ := rabbitmqCluster.AdminSecret()
			testLabels(adminSecret.Labels)
		})
	})
})
