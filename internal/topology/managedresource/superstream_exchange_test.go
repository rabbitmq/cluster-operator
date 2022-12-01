package managedresource_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1alpha1 "github.com/rabbitmq/cluster-operator/api/v1alpha1"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/topology/managedresource"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("SuperstreamExchange", func() {
	var (
		superStream     rabbitmqv1alpha1.SuperStream
		builder         *managedresource.Builder
		exchangeBuilder *managedresource.SuperStreamExchangeBuilder
		exchange        *rabbitmqv1beta1.Exchange
		scheme          *runtime.Scheme
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
		Expect(rabbitmqv1alpha1.AddToScheme(scheme)).To(Succeed())
		superStream = rabbitmqv1alpha1.SuperStream{}
		superStream.Namespace = "foo"
		superStream.Name = "foo"
		builder = &managedresource.Builder{
			ObjectOwner: &superStream,
			Scheme:      scheme,
		}
		exchangeBuilder = builder.SuperStreamExchange("vvv", testRabbitmqClusterReference)
		obj, _ := exchangeBuilder.Build()
		exchange = obj.(*rabbitmqv1beta1.Exchange)
	})

	Context("Build", func() {
		It("generates an exchange object with the correct name", func() {
			Expect(exchange.Name).To(Equal("foo-exchange"))
		})

		It("generates an exchange object with the correct namespace", func() {
			Expect(exchange.Namespace).To(Equal(superStream.Namespace))
		})

		It("sets labels on the object to tie back to the original super stream", func() {
			Expect(exchange.ObjectMeta.Labels).To(HaveKeyWithValue("rabbitmq.com/super-stream", "foo"))
		})
	})

	Context("Update", func() {
		BeforeEach(func() {
			Expect(exchangeBuilder.Update(exchange)).To(Succeed())
		})
		It("sets owner reference", func() {
			Expect(exchange.OwnerReferences[0].Name).To(Equal(superStream.Name))
		})

		It("uses the name of the super stream as the name of the exchange", func() {
			Expect(exchange.Spec.Name).To(Equal(superStream.Name))
		})

		It("sets the vhost", func() {
			Expect(exchange.Spec.Vhost).To(Equal("vvv"))
		})

		It("generates a durable exchange", func() {
			Expect(exchange.Spec.Durable).To(BeTrue())
		})

		It("sets the expected RabbitmqClusterReference", func() {
			Expect(exchange.Spec.RabbitmqClusterReference.Name).To(Equal(testRabbitmqClusterReference.Name))
			Expect(exchange.Spec.RabbitmqClusterReference.Namespace).To(Equal(testRabbitmqClusterReference.Namespace))
		})
	})
})
