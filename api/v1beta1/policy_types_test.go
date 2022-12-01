package v1beta1

import (
	"context"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Policy", func() {
	var (
		namespace = "default"
		ctx       = context.Background()
	)

	It("creates a policy with minimal configurations", func() {
		policy := Policy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-policy",
				Namespace: namespace,
			},
			Spec: PolicySpec{
				Name:    "test-policy",
				Pattern: "a-queue-name",
				Definition: &runtime.RawExtension{
					Raw: []byte(`{"key":"value"}`),
				},
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "some-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &policy)).To(Succeed())
		fetched := &Policy{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      policy.Name,
			Namespace: policy.Namespace,
		}, fetched)).To(Succeed())
		Expect(fetched.Spec.RabbitmqClusterReference).To(Equal(RabbitmqClusterReference{
			Name: "some-cluster",
		}))
		Expect(fetched.Spec.Name).To(Equal("test-policy"))
		Expect(fetched.Spec.Vhost).To(Equal("/"))
		Expect(fetched.Spec.Pattern).To(Equal("a-queue-name"))
		Expect(fetched.Spec.ApplyTo).To(Equal("all"))
		Expect(fetched.Spec.Priority).To(Equal(0))
		Expect(fetched.Spec.Definition.Raw).To(Equal([]byte(`{"key":"value"}`)))
	})

	It("creates policy with configurations", func() {
		policy := Policy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "random-policy",
				Namespace: namespace,
			},
			Spec: PolicySpec{
				Name:     "test-policy",
				Vhost:    "/hello",
				Pattern:  "*.",
				ApplyTo:  "exchanges",
				Priority: 100,
				Definition: &runtime.RawExtension{
					Raw: []byte(`{"key":"value"}`),
				},
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "random-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &policy)).To(Succeed())
		fetched := &Policy{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      policy.Name,
			Namespace: policy.Namespace,
		}, fetched)).To(Succeed())

		Expect(fetched.Spec.Name).To(Equal("test-policy"))
		Expect(fetched.Spec.Vhost).To(Equal("/hello"))
		Expect(fetched.Spec.Pattern).To(Equal("*."))
		Expect(fetched.Spec.ApplyTo).To(Equal("exchanges"))
		Expect(fetched.Spec.Priority).To(Equal(100))
		Expect(fetched.Spec.RabbitmqClusterReference).To(Equal(
			RabbitmqClusterReference{
				Name: "random-cluster",
			}))
		Expect(fetched.Spec.Definition.Raw).To(Equal([]byte(`{"key":"value"}`)))
	})

	When("creating a policy with an invalid 'ApplyTo' value", func() {
		It("fails with validation errors", func() {
			policy := Policy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid",
					Namespace: namespace,
				},
				Spec: PolicySpec{
					Name:    "test-policy",
					Pattern: "a-queue-name",
					Definition: &runtime.RawExtension{
						Raw: []byte(`{"key":"value"}`),
					},
					ApplyTo: "yo-yo",
					RabbitmqClusterReference: RabbitmqClusterReference{
						Name: "some-cluster",
					},
				},
			}
			Expect(k8sClient.Create(ctx, &policy)).To(HaveOccurred())
			Expect(k8sClient.Create(ctx, &policy)).To(MatchError(`Policy.rabbitmq.com "invalid" is invalid: spec.applyTo: Unsupported value: "yo-yo": supported values: "queues", "exchanges", "all"`))
		})
	})

})
