package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Conditions", func() {
	Describe("Ready", func() {
		It("returns 'Ready' condition set to true", func() {
			c := Ready(nil)
			Expect(string(c.Type)).To(Equal("Ready"))
			Expect(c.Status).To(Equal(corev1.ConditionTrue))
			Expect(c.Reason).To(Equal("SuccessfulCreateOrUpdate"))
			Expect(c.LastTransitionTime.IsZero()).To(BeFalse())
		})
	})
	Describe("NotReady", func() {
		It("returns 'Ready' condition set to false", func() {
			c := NotReady("fail to declare queue", nil)
			Expect(string(c.Type)).To(Equal("Ready"))
			Expect(c.Status).To(Equal(corev1.ConditionFalse))
			Expect(c.Reason).To(Equal("FailedCreateOrUpdate"))
			Expect(c.Message).To(Equal("fail to declare queue"))
			Expect(c.LastTransitionTime.IsZero()).To(BeFalse())
		})
	})
	Context("LastTransitionTime", func() {
		It("changes only if status changes", func() {
			c1 := Ready(nil)
			Expect(c1.LastTransitionTime.IsZero()).To(BeFalse())
			c2 := Ready([]Condition{
				Condition{Type: "I'm some other type"},
				c1,
			})
			Expect(c2.LastTransitionTime.Time).To(BeTemporally("==", c1.LastTransitionTime.Time))
			c3 := NotReady("some message", []Condition{c2})
			Expect(c3.LastTransitionTime.Time).To(BeTemporally(">", c2.LastTransitionTime.Time))
		})
	})
})
