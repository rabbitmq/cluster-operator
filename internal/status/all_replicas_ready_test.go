package status_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqstatus "github.com/pivotal/rabbitmq-for-kubernetes/internal/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("AllReplicasReady", func() {
	var (
		sts               *appsv1.StatefulSet
		existingCondition *rabbitmqstatus.RabbitmqClusterCondition
	)

	BeforeEach(func() {
		sts = &appsv1.StatefulSet{
			Status: appsv1.StatefulSetStatus{},
		}
		existingCondition = nil
	})

	When("all replicas are ready", func() {
		BeforeEach(func() {
			sts.Status.ReadyReplicas = 5
			sts.Status.Replicas = 5
		})

		It("returns the expected condition", func() {
			condition := rabbitmqstatus.AllReplicasReadyCondition([]runtime.Object{&corev1.Endpoints{}, sts}, existingCondition)

			By("having status true and reason message", func() {
				Expect(condition.Status).To(Equal(corev1.ConditionTrue))
				Expect(condition.Reason).To(Equal("AllPodsAreReady"))
			})
		})
	})

	When("some replicas are not ready", func() {
		BeforeEach(func() {
			sts.Status.ReadyReplicas = 3
			sts.Status.Replicas = 5
		})

		It("returns a condition with state false", func() {
			condition := rabbitmqstatus.AllReplicasReadyCondition([]runtime.Object{sts}, existingCondition)

			By("having status false and reason", func() {
				Expect(condition.Status).To(Equal(corev1.ConditionFalse))
				Expect(condition.Reason).To(Equal("NotAllPodsReady"))
				Expect(condition.Message).ToNot(BeEmpty())
			})
		})
	})

	When("the StatefulSet is not found", func() {
		BeforeEach(func() {
			sts = nil
		})

		It("returns a condition with state unknown", func() {
			condition := rabbitmqstatus.AllReplicasReadyCondition([]runtime.Object{sts}, existingCondition)

			By("having status false and reason", func() {
				Expect(condition.Status).To(Equal(corev1.ConditionUnknown))
				Expect(condition.Reason).To(Equal("MissingStatefulSet"))
				Expect(condition.Message).ToNot(BeEmpty())
			})
		})
	})

	Context("previous condition was false", func() {
		BeforeEach(func() {
			existingCondition = &rabbitmqstatus.RabbitmqClusterCondition{
				Status: corev1.ConditionFalse,
				LastTransitionTime: metav1.Time{
					Time: time.Now().Add(time.Duration(-10 * time.Second)),
				},
			}
		})

		When("all replicas become ready", func() {
			BeforeEach(func() {
				sts.Status.ReadyReplicas = 5
				sts.Status.Replicas = 5
			})

			It("sets status true, reason and updates transition time", func() {
				condition := rabbitmqstatus.AllReplicasReadyCondition([]runtime.Object{sts}, existingCondition)

				Expect(existingCondition).NotTo(BeNil())
				existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()

				Expect(condition.Status).To(Equal(corev1.ConditionTrue))
				Expect(condition.Reason).To(Equal("AllPodsAreReady"))
				Expect(condition.LastTransitionTime).ToNot(Equal(existingConditionTime))
				Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
			})
		})

		When("some replicas are not ready", func() {
			BeforeEach(func() {
				sts.Status.ReadyReplicas = 3
				sts.Status.Replicas = 5
			})

			It("sets status false, reason and does not update transition time", func() {
				condition := rabbitmqstatus.AllReplicasReadyCondition([]runtime.Object{sts}, existingCondition)

				Expect(existingCondition).NotTo(BeNil())
				existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()

				Expect(condition.Status).To(Equal(corev1.ConditionFalse))
				Expect(condition.Reason).To(Equal("NotAllPodsReady"))
				Expect(condition.LastTransitionTime).To(Equal(existingConditionTime))
			})
		})
	})
})
