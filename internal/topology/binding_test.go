package internal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/topology"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Binding", func() {
	var binding *rabbitmqv1beta1.Binding
	Context("GenerateBindingInfo", func() {
		BeforeEach(func() {
			binding = &rabbitmqv1beta1.Binding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exchange",
				},
				Spec: rabbitmqv1beta1.BindingSpec{
					Vhost:           "/avhost",
					Source:          "test-exchange",
					Destination:     "test-queue",
					DestinationType: "queue",
					RoutingKey:      "a-key",
				},
			}
		})

		It("sets the correct vhost", func() {
			info, err := internal.GenerateBindingInfo(binding)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Vhost).To(Equal("/avhost"))
		})

		It("sets the correct source", func() {
			info, err := internal.GenerateBindingInfo(binding)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Source).To(Equal("test-exchange"))
		})

		It("sets the correct destination", func() {
			info, err := internal.GenerateBindingInfo(binding)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Destination).To(Equal("test-queue"))
		})

		It("sets the correct destination type", func() {
			info, err := internal.GenerateBindingInfo(binding)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.DestinationType).To(Equal("queue"))
		})

		It("sets the correct routing key", func() {
			info, err := internal.GenerateBindingInfo(binding)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.RoutingKey).To(Equal("a-key"))
		})

		When("exchange arguments are provided", func() {
			It("generates the correct exchange arguments", func() {
				binding.Spec.Arguments = &runtime.RawExtension{
					Raw: []byte(`{"argument": "argument-value"}`),
				}
				info, err := internal.GenerateBindingInfo(binding)
				Expect(err).NotTo(HaveOccurred())
				Expect(info.Arguments).To(HaveLen(1))
				Expect(info.Arguments).To(HaveKeyWithValue("argument", "argument-value"))
			})
		})
	})

	Context("GeneratePropertiesKey", func() {
		BeforeEach(func() {
			binding = &rabbitmqv1beta1.Binding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exchange",
				},
				Spec: rabbitmqv1beta1.BindingSpec{
					Vhost:           "/avhost",
					Source:          "test-exchange",
					Destination:     "test-queue",
					DestinationType: "queue",
				},
			}
		})

		When("routing key is not set", func() {
			It("returns the default properties key value", func() {
				propertiesKey := internal.GeneratePropertiesKey(binding)
				Expect(propertiesKey).To(Equal("~"))
			})
		})

		When("routing key is set", func() {
			It("returns the routing key as properties key", func() {
				binding.Spec.RoutingKey = "a-great-routing-key"
				propertiesKey := internal.GeneratePropertiesKey(binding)
				Expect(propertiesKey).To(Equal("a-great-routing-key"))
			})

			It("replaces character '~' if it's in the routing key", func() {
				binding.Spec.RoutingKey = "special~routing~key"
				propertiesKey := internal.GeneratePropertiesKey(binding)
				Expect(propertiesKey).To(Equal("special%7Erouting%7Ekey"))
			})
		})
	})
})
