package resource_test

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("RegistrySecret", func() {
	var (
		secret                 *corev1.Secret
		instance               rabbitmqv1beta1.RabbitmqCluster
		rabbitmqCluster        *resource.RabbitmqResourceBuilder
		registrySecretBuilder  *resource.RegistrySecretBuilder
		operatorRegistrySecret *corev1.Secret
	)

	BeforeEach(func() {
		operatorRegistrySecret = &corev1.Secret{
			Data: map[string][]byte{},
			Type: "Opaque",
		}

		instance = rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: v1.ObjectMeta{
				Name:      "registry-rabbit",
				Namespace: "a namespace",
			},
		}
		rabbitmqCluster = &resource.RabbitmqResourceBuilder{
			Instance: &instance,
			DefaultConfiguration: resource.DefaultConfiguration{
				OperatorRegistrySecret: operatorRegistrySecret,
			},
		}
		registrySecretBuilder = rabbitmqCluster.RegistrySecret()
	})

	Context("Build", func() {
		BeforeEach(func() {
			obj, err := registrySecretBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			secret = obj.(*corev1.Secret)
		})

		It("creates the secret with correct name and namespace", func() {
			Expect(secret.Name).To(Equal("registry-rabbit-registry-access"))
			Expect(secret.Namespace).To(Equal("a namespace"))
		})

		It("creates the secret with the same type as operator registry secret", func() {
			Expect(secret.Type).To(Equal(operatorRegistrySecret.Type))
		})

		It("only creates the required labels", func() {
			labels := secret.Labels
			Expect(len(labels)).To(Equal(3))
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
			Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
		})
	})

	When("no operator registry secret is set in the default configuration", func() {
		BeforeEach(func() {
			rabbitmqCluster = &resource.RabbitmqResourceBuilder{
				Instance: &instance,
				DefaultConfiguration: resource.DefaultConfiguration{
					OperatorRegistrySecret: nil,
				},
			}
			registrySecretBuilder = rabbitmqCluster.RegistrySecret()
		})

		It("does not create the secret", func() {
			obj, err := registrySecretBuilder.Build()
			Expect(obj).To(BeNil())
			Expect(err).NotTo(HaveOccurred())
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

			obj, err := registrySecretBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			secret = obj.(*corev1.Secret)
		})

		It("has the labels from the CRD on the admin secret", func() {
			testLabels(secret.Labels)
		})

		It("also has the required labels", func() {
			labels := secret.Labels
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
			Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
		})
	})

	Context("Build with annotations on CR", func() {
		BeforeEach(func() {
			instance.Annotations = map[string]string{
				"my-annotation":              "i-like-this",
				"kubernetes.io/name":         "i-do-not-like-this",
				"kubectl.kubernetes.io/name": "i-do-not-like-this",
				"k8s.io/name":                "i-do-not-like-this",
			}

			obj, err := registrySecretBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			secret = obj.(*corev1.Secret)
		})

		It("has the annotations from the CRD on the registry secret", func() {
			testAnnotations(secret.Annotations)
		})
	})

	Context("Update", func() {
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
			err := registrySecretBuilder.Update(secret)
			Expect(err).NotTo(HaveOccurred())
		})

		It("adds labels from the CR", func() {
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
})
