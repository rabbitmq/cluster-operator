package v1beta1

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Binding spec", func() {
	var (
		namespace = "default"
		ctx       = context.Background()
	)

	It("creates a binding with default settings", func() {
		expectedSpec := BindingSpec{
			Vhost: "/",
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name: "some-cluster",
			},
		}

		binding := Binding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-binding",
				Namespace: namespace,
			},
			Spec: BindingSpec{
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "some-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &binding)).To(Succeed())
		fetchedBinding := &Binding{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      binding.Name,
			Namespace: binding.Namespace,
		}, fetchedBinding)).To(Succeed())
		Expect(fetchedBinding.Spec).To(Equal(expectedSpec))
	})

	It("creates a binding with configurations", func() {
		binding := Binding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "random-binding",
				Namespace: namespace,
			},
			Spec: BindingSpec{
				Vhost:           "/avhost",
				Source:          "anexchange",
				Destination:     "aqueue",
				DestinationType: "queue",
				RoutingKey:      "akey",
				Arguments: &runtime.RawExtension{
					Raw: []byte(`{"argument":"argument-value"}`),
				},
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "random-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &binding)).To(Succeed())
		fetchedBinding := &Binding{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      binding.Name,
			Namespace: binding.Namespace,
		}, fetchedBinding)).To(Succeed())

		Expect(fetchedBinding.Spec.Vhost).To(Equal("/avhost"))
		Expect(fetchedBinding.Spec.Source).To(Equal("anexchange"))
		Expect(fetchedBinding.Spec.Destination).To(Equal("aqueue"))
		Expect(fetchedBinding.Spec.DestinationType).To(Equal("queue"))
		Expect(fetchedBinding.Spec.RoutingKey).To(Equal("akey"))
		Expect(fetchedBinding.Spec.RabbitmqClusterReference).To(Equal(
			RabbitmqClusterReference{
				Name: "random-cluster",
			}))
		Expect(fetchedBinding.Spec.Arguments.Raw).To(Equal([]byte(`{"argument":"argument-value"}`)))
	})
})
