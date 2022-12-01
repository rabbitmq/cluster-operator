package internal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/topology"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GenerateVhostSettings", func() {
	var v *rabbitmqv1beta1.Vhost

	BeforeEach(func() {
		v = &rabbitmqv1beta1.Vhost{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: rabbitmqv1beta1.VhostSpec{
				Tracing: true,
				Tags:    []string{"tag1", "tag2", "multi_dc_replication"},
			},
		}
	})

	It("sets 'tracing' according to vhost.spec", func() {
		settings := internal.GenerateVhostSettings(v)
		Expect(settings.Tracing).To(BeTrue())
	})

	It("sets 'tags' according to vhost.spec.tags", func() {
		settings := internal.GenerateVhostSettings(v)
		Expect(settings.Tags).To(ConsistOf("tag1", "tag2", "multi_dc_replication"))
	})
})
