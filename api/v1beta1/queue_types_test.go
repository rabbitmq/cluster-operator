package v1beta1

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Queue spec", func() {
	var (
		namespace = "default"
		ctx       = context.Background()
	)

	It("creates a queue with default settings", func() {
		expectedSpec := QueueSpec{
			Name:       "test-queue",
			Vhost:      "/",
			Durable:    false,
			AutoDelete: false,
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name: "some-cluster",
			},
		}

		q := Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-queue",
				Namespace: namespace,
			},
			Spec: QueueSpec{
				Name: "test-queue",
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "some-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &q)).To(Succeed())
		fetchedQ := &Queue{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      q.Name,
			Namespace: q.Namespace,
		}, fetchedQ)).To(Succeed())
		Expect(fetchedQ.Spec).To(Equal(expectedSpec))
	})

	It("creates a queue with configurations", func() {
		q := Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "random-q",
				Namespace: namespace,
			},
			Spec: QueueSpec{
				Name:       "test-queue",
				Vhost:      "/hello",
				Type:       "a type",
				Durable:    true,
				AutoDelete: true,
				Arguments: &runtime.RawExtension{
					Raw: []byte(`{"yoyo":10}`),
				},
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "random-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &q)).To(Succeed())
		fetchedQ := &Queue{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      q.Name,
			Namespace: q.Namespace,
		}, fetchedQ)).To(Succeed())

		Expect(fetchedQ.Spec.Name).To(Equal("test-queue"))
		Expect(fetchedQ.Spec.Vhost).To(Equal("/hello"))
		Expect(fetchedQ.Spec.Type).To(Equal("a type"))
		Expect(fetchedQ.Spec.Durable).To(BeTrue())
		Expect(fetchedQ.Spec.AutoDelete).To(BeTrue())
		Expect(fetchedQ.Spec.RabbitmqClusterReference).To(Equal(
			RabbitmqClusterReference{
				Name: "random-cluster",
			}))
		Expect(fetchedQ.Spec.Arguments.Raw).To(Equal([]byte(`{"yoyo":10}`)))
	})
})
