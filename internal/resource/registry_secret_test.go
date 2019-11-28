package resource_test

import (
	. "github.com/onsi/ginkgo"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("RegistrySecret", func() {
	var (
		instance             rabbitmqv1beta1.RabbitmqCluster
		scheme               *runtime.Scheme
		defaultConfiguration resource.DefaultConfiguration
		cluster              *resource.RabbitmqResourceBuilder
	)

	Context("label inheritance", func() {
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

			scheme = runtime.NewScheme()
			rabbitmqv1beta1.AddToScheme(scheme)
			defaultscheme.AddToScheme(scheme)

			defaultConfiguration = resource.DefaultConfiguration{
				Scheme:                 scheme,
				OperatorRegistrySecret: &corev1.Secret{},
			}
			cluster = &resource.RabbitmqResourceBuilder{
				Instance:             &instance,
				DefaultConfiguration: defaultConfiguration,
			}
		})

		It("has the labels from the CRD on the registry secret", func() {
			registrySecret := cluster.RegistrySecret()
			testLabels(registrySecret.Labels)
		})
	})
})
