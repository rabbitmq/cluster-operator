package internal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	. "github.com/rabbitmq/cluster-operator/internal/topology"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("GeneratePolicy", func() {
	var p *rabbitmqv1beta1.Policy

	BeforeEach(func() {
		p = &rabbitmqv1beta1.Policy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "new-policy",
			},
			Spec: rabbitmqv1beta1.PolicySpec{
				Name:       "new-p",
				Vhost:      "/new-vhost",
				ApplyTo:    "exchanges",
				Pattern:    "exchange-name",
				Priority:   5,
				Definition: &runtime.RawExtension{Raw: []byte(`{"key":"value"}`)},
			},
		}
	})

	It("sets policy name according to policySpec", func() {
		generated, err := GeneratePolicy(p)
		Expect(err).NotTo(HaveOccurred())
		Expect(generated.Name).To(Equal("new-p"))
	})

	It("sets policy vhost according to policySpec", func() {
		generated, err := GeneratePolicy(p)
		Expect(err).NotTo(HaveOccurred())
		Expect(generated.Vhost).To(Equal("/new-vhost"))
	})

	It("sets 'ApplyTo' according to policySpec", func() {
		generated, err := GeneratePolicy(p)
		Expect(err).NotTo(HaveOccurred())
		Expect(generated.ApplyTo).To(Equal("exchanges"))
	})

	It("sets 'priority' according to policySpec", func() {
		generated, err := GeneratePolicy(p)
		Expect(err).NotTo(HaveOccurred())
		Expect(generated.Priority).To(Equal(5))
	})

	It("sets 'pattern' according to policySpec", func() {
		generated, err := GeneratePolicy(p)
		Expect(err).NotTo(HaveOccurred())
		Expect(generated.Pattern).To(Equal("exchange-name"))
	})

	It("sets definition according to policySpec", func() {
		generated, err := GeneratePolicy(p)
		Expect(err).NotTo(HaveOccurred())
		Expect(generated.Definition).Should(HaveLen(1))
		Expect(generated.Definition).Should(HaveKeyWithValue("key", "value"))
	})
})
