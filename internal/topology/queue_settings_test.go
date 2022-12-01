package internal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/topology"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("GenerateQueueSettings", func() {
	var q *rabbitmqv1beta1.Queue

	BeforeEach(func() {
		q = &rabbitmqv1beta1.Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a-queue",
			},
			Spec: rabbitmqv1beta1.QueueSpec{
				Type:       "quorum",
				AutoDelete: false,
				Durable:    true,
			},
		}
	})

	It("sets QueueSettings.Type according to queue.spec", func() {
		settings, err := internal.GenerateQueueSettings(q)
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.Type).To(Equal("quorum"))
	})

	It("sets QueueSettings.AutoDelete according to queue.spec", func() {
		settings, err := internal.GenerateQueueSettings(q)
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.AutoDelete).To(BeFalse())
	})

	It("sets QueueSettings.Durable according to queue.spec", func() {
		settings, err := internal.GenerateQueueSettings(q)
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.Durable).To(BeTrue())
	})

	When("queue arguments are provided", func() {
		It("generates the correct queue arguments", func() {
			q.Spec.Arguments = &runtime.RawExtension{
				Raw: []byte(`{"x-delivery-limit": 10000,
"x-max-in-memory-length": 500,
"x-max-in-memory-bytes": 5000,
"x-max-length": 300,
"x-max-length-bytes": 60000,
"x-dead-letter-exchange": "test",
"x-single-active-consumer": true
}`)}
			settings, err := internal.GenerateQueueSettings(q)
			Expect(err).NotTo(HaveOccurred())
			Expect(settings.Arguments).Should(SatisfyAll(
				HaveLen(7),
				// GenerateQueueSettings Unmarshal queue.Spec.Arguments
				// Unmarshall stores float64 for JSON numbers
				HaveKeyWithValue("x-delivery-limit", float64(10000)),
				HaveKeyWithValue("x-max-in-memory-length", float64(500)),
				HaveKeyWithValue("x-max-in-memory-bytes", float64(5000)),
				HaveKeyWithValue("x-max-length", float64(300)),
				HaveKeyWithValue("x-max-length-bytes", float64(60000)),
				HaveKeyWithValue("x-dead-letter-exchange", "test"),
				HaveKeyWithValue("x-single-active-consumer", true),
			))
		})
	})
})
