package status_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqstatus "github.com/pivotal/rabbitmq-for-kubernetes/internal/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("AllNodesAvailable", func() {
	var (
		sts *appsv1.StatefulSet
	)

	BeforeEach(func() {
		sts = &appsv1.StatefulSet{
			Status: appsv1.StatefulSetStatus{},
		}
	})

	When("all replicas are ready", func() {
		BeforeEach(func() {
			sts.Status.ReadyReplicas = 5
			sts.Status.Replicas = 5
		})

		It("returns the expected condition", func() {
			condition := rabbitmqstatus.AllNodesAvailableCondition(sts)

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
			condition := rabbitmqstatus.AllNodesAvailableCondition(sts)

			By("having status false and reason", func() {
				Expect(condition.Status).To(Equal(corev1.ConditionFalse))
				Expect(condition.Reason).To(Equal("NotAllPodsAreReady"))
				Expect(condition.Message).ToNot(BeEmpty())
			})
		})
	})

	When("the StatefulSet is not found", func() {
		BeforeEach(func() {
			sts = nil
		})

		It("returns a condition with state unknown", func() {
			condition := rabbitmqstatus.AllNodesAvailableCondition(sts)

			By("having status false and reason", func() {
				Expect(condition.Status).To(Equal(corev1.ConditionUnknown))
				Expect(condition.Reason).To(Equal("MissingStatefulSet"))
				Expect(condition.Message).ToNot(BeEmpty())
			})
		})
	})

})
