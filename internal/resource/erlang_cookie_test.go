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

var _ = Describe("ErlangCookie", func() {
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
		secret, err = rabbitmqCluster.ErlangCookie()
		Expect(err).NotTo(HaveOccurred())
	})

	It("creates the secret with the correct name and namespace", func() {
		Expect(secret.Name).To(Equal(instance.ChildResourceName("erlang-cookie")))
		Expect(secret.Namespace).To(Equal(instance.Namespace))
	})

	It("creates the secret required labels", func() {
		labels := secret.Labels
		Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
		Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
		Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
	})

	It("creates a 'opaque' secret ", func() {
		Expect(secret.Type).To(Equal(corev1.SecretTypeOpaque))
	})

	It("creates an erlang cookie that is base64 encoded and 24 characters", func() {
		cookie, ok := secret.Data[".erlang.cookie"]
		Expect(ok).NotTo(BeFalse())
		decodedCookie, err := b64.URLEncoding.DecodeString(string(cookie))
		Expect(err).NotTo(HaveOccurred())
		Expect(len(decodedCookie)).To(Equal(24))
	})
})
