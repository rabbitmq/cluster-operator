package status_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/pivotal/rabbitmq-for-kubernetes/internal/status"
)

var _ = Describe("Reconcilable", func() {

	It("has the required fields", func() {
		reconcilableCondition := ReconcilableCondition(corev1.ConditionTrue, "GreatSuccess")
		Expect(reconcilableCondition.Type).To(Equal(RabbitmqClusterConditionType("Reconcilable")))
		Expect(reconcilableCondition.Status).To(Equal(corev1.ConditionStatus("True")))
		Expect(reconcilableCondition.Reason).To(Equal("GreatSuccess"))
		emptyTime := metav1.Time{}
		Expect(reconcilableCondition.LastTransitionTime).NotTo(Equal(emptyTime))
	})

	Context("Transition time", func() {

		var (
			existingCondition     *RabbitmqClusterCondition
			previousConditionTime time.Time = time.Date(2020, 2, 2, 8, 0, 0, 0, time.UTC)
		)

		Context("Previous condition was true", func() {
			BeforeEach(func() {
				existingCondition = &RabbitmqClusterCondition{
					Status: corev1.ConditionTrue,
					LastTransitionTime: metav1.Time{
						Time: previousConditionTime,
					},
				}
			})

			When("remains true", func() {
				It("does not update transition time", func() {
					condition := ReconcilableCondition(corev1.ConditionTrue, "GreatSuccess", existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeTrue(),
						"Condition time does not match previousConditionTime. Previous: %v Current: %v",
						existingConditionTime, condition.LastTransitionTime.Time)
				})
			})

			When("transitions from true to false", func() {
				It("updates the transition time", func() {
					condition := ReconcilableCondition(corev1.ConditionFalse, "SomeError", existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()

					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse(),
						"Condition time did not update. Previous: %v Current: %v",
						existingConditionTime, condition.LastTransitionTime)
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse(),
						"Condition time is in the past. Previous: %v Current: %v",
						existingConditionTime, condition.LastTransitionTime)
				})
			})

			When("transitions from true to unknown", func() {
				It("updates the transition time", func() {
					condition := ReconcilableCondition(corev1.ConditionUnknown, "NoClue", existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()

					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse(),
						"Condition time did not update. Previous: %v Current: %v",
						existingConditionTime, condition.LastTransitionTime)
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse(),
						"Condition time is in the past. Previous: %v Current: %v",
						existingConditionTime, condition.LastTransitionTime)
				})
			})
		})

		Context("Previous condition was false", func() {
			BeforeEach(func() {
				existingCondition = &RabbitmqClusterCondition{
					Status: corev1.ConditionFalse,
					LastTransitionTime: metav1.Time{
						Time: previousConditionTime,
					},
				}
			})

			When("remains false", func() {
				It("does not update transition time", func() {
					condition := ReconcilableCondition(corev1.ConditionFalse, "SomeError", existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeTrue(),
						"Condition time does not match previousConditionTime. Previous: %v Current: %v",
						existingConditionTime, condition.LastTransitionTime.Time)
				})
			})

			When("transitions from false to true", func() {
				It("updates the transition time", func() {
					condition := ReconcilableCondition(corev1.ConditionTrue, "AllGoodHere", existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()

					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse(),
						"Condition time did not update. Previous: %v Current: %v",
						existingConditionTime, condition.LastTransitionTime)
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse(),
						"Condition time is in the past. Previous: %v Current: %v",
						existingConditionTime, condition.LastTransitionTime)
				})
			})

			When("transitions from false to unknown", func() {
				It("updates the transition time", func() {
					condition := ReconcilableCondition(corev1.ConditionUnknown, "NotSure", existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()

					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse(),
						"Condition time did not update. Previous: %v Current: %v",
						existingConditionTime, condition.LastTransitionTime)
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse(),
						"Condition time is in the past. Previous: %v Current: %v",
						existingConditionTime, condition.LastTransitionTime)
				})
			})
		})

		Context("Previous condition was unknown", func() {
			BeforeEach(func() {
				existingCondition = &RabbitmqClusterCondition{
					Status: corev1.ConditionUnknown,
					LastTransitionTime: metav1.Time{
						Time: previousConditionTime,
					},
				}
			})

			When("remains unknown", func() {
				It("does not update transition time", func() {
					condition := ReconcilableCondition(corev1.ConditionUnknown, "WhoKnows", existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeTrue(),
						"Condition time does not match previousConditionTime. Previous: %v Current: %v",
						existingConditionTime, condition.LastTransitionTime.Time)
				})
			})

			When("transitions from unkown to false", func() {
				It("updates the transition time", func() {
					condition := ReconcilableCondition(corev1.ConditionFalse, "SomeError", existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()

					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse(),
						"Condition time did not update. Previous: %v Current: %v",
						existingConditionTime, condition.LastTransitionTime)
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse(),
						"Condition time is in the past. Previous: %v Current: %v",
						existingConditionTime, condition.LastTransitionTime)
				})
			})

			When("transitions from unkown to true", func() {
				It("updates the transition time", func() {
					condition := ReconcilableCondition(corev1.ConditionTrue, "AllGoodHere", existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()

					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse(),
						"Condition time did not update. Previous: %v Current: %v",
						existingConditionTime, condition.LastTransitionTime)
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse(),
						"Condition time is in the past. Previous: %v Current: %v",
						existingConditionTime, condition.LastTransitionTime)
				})
			})
		})
	})

})
