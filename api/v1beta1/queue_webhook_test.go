package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("queue webhook", func() {

	var queue = Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-queue",
		},
		Spec: QueueSpec{
			Name:       "test",
			Vhost:      "/a-vhost",
			Type:       "quorum",
			Durable:    false,
			AutoDelete: true,
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name: "some-cluster",
			},
		},
	}

	Context("ValidateCreate", func() {
		It("does not allow both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret be configured", func() {
			notAllowedQ := queue.DeepCopy()
			notAllowedQ.Spec.RabbitmqClusterReference.ConnectionSecret = &corev1.LocalObjectReference{Name: "some-secret"}
			Expect(apierrors.IsForbidden(notAllowedQ.ValidateCreate())).To(BeTrue())
		})

		It("spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret cannot both be empty", func() {
			notAllowedQ := queue.DeepCopy()
			notAllowedQ.Spec.RabbitmqClusterReference.Name = ""
			notAllowedQ.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			Expect(apierrors.IsForbidden(notAllowedQ.ValidateCreate())).To(BeTrue())
		})

		It("does not allow non-durable quorum queues", func() {
			notAllowedQ := queue.DeepCopy()
			notAllowedQ.Spec.AutoDelete = false
			Expect(apierrors.IsForbidden(notAllowedQ.ValidateCreate())).To(BeTrue(), "Expected 'forbidden' response for non-durable quorum queue")
		})
	})

	Context("ValidateUpdate", func() {
		It("does not allow updates on queue name", func() {
			newQueue := queue.DeepCopy()
			newQueue.Spec.Name = "new-name"
			Expect(apierrors.IsForbidden(newQueue.ValidateUpdate(&queue))).To(BeTrue())
		})

		It("does not allow updates on vhost", func() {
			newQueue := queue.DeepCopy()
			newQueue.Spec.Vhost = "/new-vhost"
			Expect(apierrors.IsForbidden(newQueue.ValidateUpdate(&queue))).To(BeTrue())
		})

		It("does not allow updates on rabbitmqClusterReference.name", func() {
			newQueue := queue.DeepCopy()
			newQueue.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
				Name: "new-cluster",
			}
			Expect(apierrors.IsForbidden(newQueue.ValidateUpdate(&queue))).To(BeTrue())
		})

		It("does not allow updates on rabbitmqClusterReference.namespace", func() {
			newQueue := queue.DeepCopy()
			newQueue.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
				Namespace: "new-ns",
			}
			Expect(apierrors.IsForbidden(newQueue.ValidateUpdate(&queue))).To(BeTrue())
		})

		It("does not allow updates on rabbitmqClusterReference.connectionSecret", func() {
			connectionScrQ := Queue{
				ObjectMeta: metav1.ObjectMeta{
					Name: "connect-test-queue",
				},
				Spec: QueueSpec{
					Name: "test",
					RabbitmqClusterReference: RabbitmqClusterReference{
						ConnectionSecret: &corev1.LocalObjectReference{
							Name: "a-secret",
						},
					},
				},
			}
			newQueue := connectionScrQ.DeepCopy()
			newQueue.Spec.RabbitmqClusterReference.ConnectionSecret.Name = "new-secret"
			Expect(apierrors.IsForbidden(newQueue.ValidateUpdate(&connectionScrQ))).To(BeTrue())
		})

		It("does not allow updates on queue type", func() {
			newQueue := queue.DeepCopy()
			newQueue.Spec.Type = "classic"
			Expect(apierrors.IsInvalid(newQueue.ValidateUpdate(&queue))).To(BeTrue())
		})

		It("does not allow updates on durable", func() {
			newQueue := queue.DeepCopy()
			newQueue.Spec.Durable = true
			Expect(apierrors.IsInvalid(newQueue.ValidateUpdate(&queue))).To(BeTrue())
		})

		It("does not allow updates on autoDelete", func() {
			newQueue := queue.DeepCopy()
			newQueue.Spec.AutoDelete = false
			Expect(apierrors.IsInvalid(newQueue.ValidateUpdate(&queue))).To(BeTrue())
		})
	})
})
