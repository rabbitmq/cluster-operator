package resource_test

import (
	b64 "encoding/base64"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("ErlangCookie", func() {
	var (
		secret          *corev1.Secret
		instance        rabbitmqv1beta1.RabbitmqCluster
		scheme          *runtime.Scheme
		rabbitmqCluster *resource.RabbitmqResourceBuilder
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
		scheme = runtime.NewScheme()
		Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
		Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())
		erlangCookieBuilder := rabbitmqCluster.ErlangCookie()
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

	Context("Update", func() {
		var (
			secret        *corev1.Secret
			secretBuilder *resource.ErlangCookieBuilder
		)

		BeforeEach(func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{}
			instance.Namespace = "foo"
			instance.Name = "foo"
			instance.Labels = map[string]string{
				"app.kubernetes.io/foo": "bar",
				"foo":                   "bar",
				"rabbitmq":              "is-great",
				"foo/app.kubernetes.io": "edgecase",
			}

			rabbitmqCluster = &resource.RabbitmqResourceBuilder{
				Instance: &instance,
			}

			secretBuilder = rabbitmqCluster.ErlangCookie()
			obj, _ := secretBuilder.Build()
			secret = obj.(*corev1.Secret)
		})

		It("adds labels from the CRD on the headless secret", func() {
			err := secretBuilder.Update(secret)
			Expect(err).NotTo(HaveOccurred())

			testLabels(secret.Labels)
		})

		It("sets nothing if the instance has no labels", func() {
			secretBuilder = rabbitmqCluster.ErlangCookie()
			secretBuilder.Instance.Labels = nil
			obj, _ := secretBuilder.Build()
			secret = obj.(*corev1.Secret)
			err := secretBuilder.Update(secret)
			Expect(err).NotTo(HaveOccurred())

			// Expect to have a set of labels that are always included
			for label := range secret.Labels {
				Expect(strings.HasPrefix(label, "app.kubernetes.io")).To(BeTrue())
			}
		})
	})

})
