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

	Context("Build with defaults", func() {
		BeforeEach(func() {
			obj, err := erlangCookieBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			secret = obj.(*corev1.Secret)
		})

		It("creates the secret with the correct name and namespace", func() {
			Expect(secret.Name).To(Equal(instance.ChildResourceName("erlang-cookie")))
			Expect(secret.Namespace).To(Equal(instance.Namespace))
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

	Context("Update with instance labels", func() {
		BeforeEach(func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rabbit-labelled",
				},
			}
			instance.Labels = map[string]string{
				"app.kubernetes.io/foo": "bar",
				"foo":                   "bar",
				"rabbitmq":              "is-great",
				"foo/app.kubernetes.io": "edgecase",
			}

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":      instance.Name,
						"app.kubernetes.io/part-of":   "pivotal-rabbitmq",
						"this-was-the-previous-label": "should-be-deleted",
					},
				},
			}
			err := erlangCookieBuilder.Update(secret)
			Expect(err).NotTo(HaveOccurred())
		})

		It("adds labels from the CRD on the admin secret", func() {
			testLabels(secret.Labels)
		})

		It("restores the default labels", func() {
			labels := secret.Labels
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
			Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
		})

		It("deletes the labels that are removed from the CR", func() {
			Expect(secret.Labels).NotTo(HaveKey("this-was-the-previous-label"))
		})
	})

	Context("Update with instance annotations", func() {
		BeforeEach(func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rabbit-labelled",
				},
			}
			instance.Annotations = map[string]string{
				"my-annotation":               "i-like-this",
				"kubernetes.io/name":          "i-do-not-like-this",
				"kubectl.kubernetes.io/name":  "i-do-not-like-this",
				"k8s.io/name":                 "i-do-not-like-this",
				"kubernetes.io/other":         "i-do-not-like-this",
				"kubectl.kubernetes.io/other": "i-do-not-like-this",
				"k8s.io/other":                "i-do-not-like-this",
			}

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"old-annotation":                "old-value",
						"im-here-to-stay.kubernetes.io": "for-a-while",
						"kubernetes.io/name":            "should-stay",
						"kubectl.kubernetes.io/name":    "should-stay",
						"k8s.io/name":                   "should-stay",
					},
				},
			}
			err := erlangCookieBuilder.Update(secret)
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates secret annotations", func() {
			expectedAnnotations := map[string]string{
				"my-annotation":                 "i-like-this",
				"old-annotation":                "old-value",
				"im-here-to-stay.kubernetes.io": "for-a-while",
				"kubernetes.io/name":            "should-stay",
				"kubectl.kubernetes.io/name":    "should-stay",
				"k8s.io/name":                   "should-stay",
			}
			Expect(secret.Annotations).To(Equal(expectedAnnotations))
		})
	})
})
