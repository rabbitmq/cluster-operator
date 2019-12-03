package resource_test

import (
	b64 "encoding/base64"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("ErlangCookie", func() {
	var (
		secret              *corev1.Secret
		instance            rabbitmqv1beta1.RabbitmqCluster
		rabbitmqCluster     *resource.RabbitmqResourceBuilder
		erlangCookieBuilder *resource.ErlangCookieBuilder
	)

	BeforeEach(func() {
		instance = rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: v1.ObjectMeta{
				Name:      "a name",
				Namespace: "a namespace",
			},
		}
		rabbitmqCluster = &resource.RabbitmqResourceBuilder{
			Instance: &instance,
		}
		erlangCookieBuilder = rabbitmqCluster.ErlangCookie()
	})

	Context("Build", func() {
		BeforeEach(func() {
			obj, err := erlangCookieBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			secret = obj.(*corev1.Secret)
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

	Context("Build with labels on CR", func() {
		BeforeEach(func() {
			instance.Labels = map[string]string{
				"app.kubernetes.io/foo": "bar",
				"foo":                   "bar",
				"rabbitmq":              "is-great",
				"foo/app.kubernetes.io": "edgecase",
			}
			obj, _ := erlangCookieBuilder.Build()
			secret = obj.(*corev1.Secret)
		})

		It("has the labels from the CRD on the erlang cookie secret", func() {
			testLabels(secret.Labels)
		})
	})

	Context("Update", func() {
		BeforeEach(func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{}
			instance.Namespace = "rabbit-labelled"
			instance.Name = "rabbit-labelled"
			instance.Labels = map[string]string{
				"app.kubernetes.io/foo": "bar",
				"foo":                   "bar",
				"rabbitmq":              "is-great",
				"foo/app.kubernetes.io": "edgecase",
			}

			rabbitmqCluster = &resource.RabbitmqResourceBuilder{
				Instance: &instance,
			}

			erlangCookieBuilder = rabbitmqCluster.ErlangCookie()
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name": "rabbit-labelled",
					},
				},
			}
			err := erlangCookieBuilder.Update(secret)
			Expect(err).NotTo(HaveOccurred())
		})

		It("adds labels from the CRD on the erlang cookie secret", func() {
			testLabels(secret.Labels)
		})

		It("persists the labels it had before Update", func() {
			Expect(secret.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "rabbit-labelled"))
		})
	})

})
