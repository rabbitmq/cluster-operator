package status_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/pivotal/rabbitmq-for-kubernetes/internal/status"
)

var _ = Describe("Reconciled", func() {

	It("has the required fields", func() {
		reconcilableCondition := ReconciledCondition(corev1.ConditionTrue, "GreatSuccess", "SomeMessage")
		Expect(reconcilableCondition.Type).To(Equal(RabbitmqClusterConditionType("Reconciled")))
		Expect(reconcilableCondition.Status).To(Equal(corev1.ConditionStatus("True")))
		Expect(reconcilableCondition.Reason).To(Equal("GreatSuccess"))
		Expect(reconcilableCondition.Message).To(Equal("SomeMessage"))
		emptyTime := metav1.Time{}
		Expect(reconcilableCondition.LastTransitionTime).NotTo(Equal(emptyTime))
	})
})
