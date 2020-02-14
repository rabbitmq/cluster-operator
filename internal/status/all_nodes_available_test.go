package status_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqstatus "github.com/pivotal/rabbitmq-for-kubernetes/internal/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("AllNodesAvailable", func() {

	var (
		childSts *appsv1.StatefulSet
	)

	BeforeEach(func() {
		childSts = &appsv1.StatefulSet{
			Status: appsv1.StatefulSetStatus{},
		}
	})

	When("all replicas are ready", func() {
		var (
			conditionManager rabbitmqstatus.AllNodesAvailableConditionManager
		)

		BeforeEach(func() {
			childSts.Status.ReadyReplicas = 5
			childSts.Status.Replicas = 5
			conditionManager = rabbitmqstatus.NewAllNodesAvailableConditionManager(childSts)
		})

		It("returns the expected condition", func() {
			condition := conditionManager.Condition()

			By("having status true and reason message", func() {
				Expect(condition.Status).To(Equal(corev1.ConditionTrue))
				Expect(condition.Reason).To(Equal("AllPodsAreReady"))
			})

			By("having a probe time", func() {
				Expect(condition.LastProbeTime).NotTo(Equal(metav1.Time{}))
			})
		})
	})

	When("some replicas are not ready", func() {
		var (
			conditionManager rabbitmqstatus.AllNodesAvailableConditionManager
		)

		BeforeEach(func() {
			childSts.Status.ReadyReplicas = 3
			childSts.Status.Replicas = 5
			conditionManager = rabbitmqstatus.NewAllNodesAvailableConditionManager(childSts)
		})

		It("returns a condition with state false", func() {
			condition := conditionManager.Condition()

			By("having status false and reason", func() {
				Expect(condition.Status).To(Equal(corev1.ConditionFalse))
				Expect(condition.Reason).To(Equal("OneOrMorePodsAreNotReady"))
				Expect(condition.Message).ToNot(BeEmpty())
			})

			By("having a probe time", func() {
				Expect(condition.LastProbeTime).NotTo(Equal(metav1.Time{}))
			})
		})
	})

	When("the StatefulSet is not found", func() {
		var (
			conditionManager rabbitmqstatus.AllNodesAvailableConditionManager
		)

		BeforeEach(func() {
			childSts = nil
			conditionManager = rabbitmqstatus.NewAllNodesAvailableConditionManager(childSts)
		})

		It("returns a condition with state unknown", func() {
			condition := conditionManager.Condition()

			By("having status false and reason", func() {
				Expect(condition.Status).To(Equal(corev1.ConditionUnknown))
				Expect(condition.Reason).To(Equal("CouldNotAccessStatefulSetStatus"))
				Expect(condition.Message).ToNot(BeEmpty())
			})

			By("having a probe time", func() {
				Expect(condition.LastProbeTime).NotTo(Equal(metav1.Time{}))
			})
		})
	})

})
