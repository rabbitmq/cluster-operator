package v1alpha1

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("SuperStream spec", func() {
	var (
		namespace = "default"
		ctx       = context.Background()
	)

	It("creates a superstream with default settings", func() {
		expectedSpec := SuperStreamSpec{
			Name:       "test-super-stream",
			Vhost:      "/",
			Partitions: 3,
			RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
				Name: "some-cluster",
			},
		}

		superStream := SuperStream{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-super-stream",
				Namespace: namespace,
			},
			Spec: SuperStreamSpec{
				Name: "test-super-stream",
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: "some-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &superStream)).To(Succeed())
		fetchedSuperStream := &SuperStream{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      superStream.Name,
			Namespace: superStream.Namespace,
		}, fetchedSuperStream)).To(Succeed())
		Expect(fetchedSuperStream.Spec).To(Equal(expectedSpec))
	})

	It("creates a superstream with specified settings", func() {
		expectedSpec := SuperStreamSpec{
			Name:       "test-super-stream2",
			Vhost:      "test-vhost",
			Partitions: 5,
			RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
				Name: "some-cluster",
			},
		}

		superStream := SuperStream{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-super-stream2",
				Namespace: namespace,
			},
			Spec: SuperStreamSpec{
				Name:       "test-super-stream2",
				Vhost:      "test-vhost",
				Partitions: 5,
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: "some-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &superStream)).To(Succeed())
		fetchedSuperStream := &SuperStream{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      superStream.Name,
			Namespace: superStream.Namespace,
		}, fetchedSuperStream)).To(Succeed())
		Expect(fetchedSuperStream.Spec).To(Equal(expectedSpec))
	})

})
