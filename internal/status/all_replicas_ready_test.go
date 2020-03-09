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
		sts                   *appsv1.StatefulSet
		existingCondition     *rabbitmqstatus.RabbitmqClusterCondition
		currentTimeFn         func() time.Time
		previousConditionTime time.Time
	)

	BeforeEach(func() {
		sts = &appsv1.StatefulSet{
			Status: appsv1.StatefulSetStatus{},
		}
		existingCondition = nil
		currentTimeFn = func() time.Time {
			return time.Date(2020, 2, 2, 9, 6, 0, 0, time.UTC)
		}
		previousConditionTime = time.Date(2020, 2, 2, 8, 0, 0, 0, time.UTC)
	})

	Context("previous condition was not set", func() {
		var (
			expectedTime metav1.Time
		)
		BeforeEach(func() {
			expectedTime = metav1.Time{
				Time: time.Date(2020, 2, 2, 9, 6, 0, 0, time.UTC),
			}
		})
		When("all replicas are ready", func() {
			BeforeEach(func() {
				sts.Status.ReadyReplicas = 5
				sts.Status.Replicas = 5
			})

			It("returns the expected condition", func() {
				condition := rabbitmqstatus.AllReplicasReadyCondition([]runtime.Object{&corev1.Endpoints{}, sts}, existingCondition, currentTimeFn)

				By("having status true and reason message", func() {
					Expect(condition.Status).To(Equal(corev1.ConditionTrue))
					Expect(condition.Reason).To(Equal("AllPodsAreReady"))
				})

				By("settings the transition time", func() {
					Expect(condition.LastTransitionTime).To(Equal(expectedTime))
				})
			})
		})

		When("some replicas are not ready", func() {
			BeforeEach(func() {
				sts.Status.ReadyReplicas = 3
				sts.Status.Replicas = 5
			})

			It("returns a condition with state false", func() {
				condition := rabbitmqstatus.AllReplicasReadyCondition([]runtime.Object{sts}, existingCondition, currentTimeFn)

				By("having status false and reason", func() {
					Expect(condition.Status).To(Equal(corev1.ConditionFalse))
					Expect(condition.Reason).To(Equal("NotAllPodsReady"))
					Expect(condition.Message).ToNot(BeEmpty())
				})

				By("settings the transition time", func() {
					Expect(condition.LastTransitionTime).To(Equal(expectedTime))
				})
			})
		})

		When("the StatefulSet is not found", func() {
			BeforeEach(func() {
				sts = nil
			})

			It("returns a condition with state unknown", func() {
				condition := rabbitmqstatus.AllReplicasReadyCondition([]runtime.Object{sts}, existingCondition, currentTimeFn)

				By("having status unknown and reason", func() {
					Expect(condition.Status).To(Equal(corev1.ConditionUnknown))
					Expect(condition.Reason).To(Equal("MissingStatefulSet"))
					Expect(condition.Message).ToNot(BeEmpty())
				})

				By("settings the transition time", func() {
					Expect(condition.LastTransitionTime).To(Equal(expectedTime))
				})
			})
		})
	})

	Context("previous condition was false", func() {
		BeforeEach(func() {
			existingCondition = &rabbitmqstatus.RabbitmqClusterCondition{
				Status: corev1.ConditionFalse,
				LastTransitionTime: metav1.Time{
					Time: previousConditionTime,
				},
			}
		})

		When("all replicas become ready", func() {
			BeforeEach(func() {
				sts.Status.ReadyReplicas = 5
				sts.Status.Replicas = 5
			})

			It("sets status true, reason and updates transition time", func() {
				condition := rabbitmqstatus.AllReplicasReadyCondition([]runtime.Object{sts}, existingCondition, currentTimeFn)

				Expect(existingCondition).NotTo(BeNil())
				existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()

				Expect(condition.Status).To(Equal(corev1.ConditionTrue))
				Expect(condition.Reason).To(Equal("AllPodsAreReady"))
				Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
				Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
			})
		})

		When("some replicas are not ready", func() {
			BeforeEach(func() {
				sts.Status.ReadyReplicas = 3
				sts.Status.Replicas = 5
			})

			It("sets status false, reason and does not update transition time", func() {
				condition := rabbitmqstatus.AllReplicasReadyCondition([]runtime.Object{sts}, existingCondition, currentTimeFn)

				Expect(existingCondition).NotTo(BeNil())
				existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()

				Expect(condition.Status).To(Equal(corev1.ConditionFalse))
				Expect(condition.Reason).To(Equal("NotAllPodsReady"))
				Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeTrue())
			})
		})

		When("the stateful set is not found", func() {
			BeforeEach(func() {
				sts = nil
			})

			It("returns a condition with state unknown and updates transition time", func() {
				condition := rabbitmqstatus.AllReplicasReadyCondition([]runtime.Object{sts}, existingCondition, currentTimeFn)

				By("having status unknown and reason", func() {
					Expect(condition.Status).To(Equal(corev1.ConditionUnknown))
					Expect(condition.Reason).To(Equal("MissingStatefulSet"))
					Expect(condition.Message).ToNot(BeEmpty())
				})

				By("updating the transition time", func() {
					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
				})
			})
		})
	})

	Context("previous condition was true", func() {
		BeforeEach(func() {
			existingCondition = &rabbitmqstatus.RabbitmqClusterCondition{
				Status: corev1.ConditionTrue,
				LastTransitionTime: metav1.Time{
					Time: previousConditionTime,
				},
			}
		})

		When("all replicas are ready", func() {
			BeforeEach(func() {
				sts.Status.ReadyReplicas = 5
				sts.Status.Replicas = 5
			})

			It("sets status true, reason and does not update transition time", func() {
				condition := rabbitmqstatus.AllReplicasReadyCondition([]runtime.Object{sts}, existingCondition, currentTimeFn)

				Expect(existingCondition).NotTo(BeNil())
				existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()

				Expect(condition.Status).To(Equal(corev1.ConditionTrue))
				Expect(condition.Reason).To(Equal("AllPodsAreReady"))
				Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeTrue())
			})
		})

		When("some replicas are not ready", func() {
			BeforeEach(func() {
				sts.Status.ReadyReplicas = 3
				sts.Status.Replicas = 5
			})

			It("sets status false, reason and updates transition time", func() {
				condition := rabbitmqstatus.AllReplicasReadyCondition([]runtime.Object{sts}, existingCondition, currentTimeFn)

				Expect(existingCondition).NotTo(BeNil())
				existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()

				Expect(condition.Status).To(Equal(corev1.ConditionFalse))
				Expect(condition.Reason).To(Equal("NotAllPodsReady"))
				Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
				Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
			})
		})

		When("the stateful set is not found", func() {
			BeforeEach(func() {
				sts = nil
			})

			It("returns a condition with state unknown and updates transition time", func() {
				condition := rabbitmqstatus.AllReplicasReadyCondition([]runtime.Object{sts}, existingCondition, currentTimeFn)

				By("having status unknown and reason", func() {
					Expect(condition.Status).To(Equal(corev1.ConditionUnknown))
					Expect(condition.Reason).To(Equal("MissingStatefulSet"))
					Expect(condition.Message).ToNot(BeEmpty())
				})

				By("updating the transition time", func() {
					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
				})
			})
		})
	})

	Context("previous condition was unknown", func() {
		BeforeEach(func() {
			existingCondition = &rabbitmqstatus.RabbitmqClusterCondition{
				Status: corev1.ConditionUnknown,
				LastTransitionTime: metav1.Time{
					Time: previousConditionTime,
				},
			}
		})

		When("all replicas are ready", func() {
			BeforeEach(func() {
				sts.Status.ReadyReplicas = 5
				sts.Status.Replicas = 5
			})

			It("sets status true, reason and updates transition time", func() {
				condition := rabbitmqstatus.AllReplicasReadyCondition([]runtime.Object{sts}, existingCondition, currentTimeFn)

				Expect(existingCondition).NotTo(BeNil())
				existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()

				Expect(condition.Status).To(Equal(corev1.ConditionTrue))
				Expect(condition.Reason).To(Equal("AllPodsAreReady"))
				Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
				Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
			})
		})

		When("some replicas are not ready", func() {
			BeforeEach(func() {
				sts.Status.ReadyReplicas = 3
				sts.Status.Replicas = 5
			})

			It("sets status false, reason and updates transition time", func() {
				condition := rabbitmqstatus.AllReplicasReadyCondition([]runtime.Object{sts}, existingCondition, currentTimeFn)

				Expect(existingCondition).NotTo(BeNil())
				existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()

				Expect(condition.Status).To(Equal(corev1.ConditionFalse))
				Expect(condition.Reason).To(Equal("NotAllPodsReady"))
				Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
				Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
			})
		})

		When("the stateful set is not found", func() {
			BeforeEach(func() {
				sts = nil
			})

			It("returns a condition with state unknown and does not update transition time", func() {
				condition := rabbitmqstatus.AllReplicasReadyCondition([]runtime.Object{sts}, existingCondition, currentTimeFn)

				By("having status unknown and reason", func() {
					Expect(condition.Status).To(Equal(corev1.ConditionUnknown))
					Expect(condition.Reason).To(Equal("MissingStatefulSet"))
					Expect(condition.Message).ToNot(BeEmpty())
				})

				By("updating the transition time", func() {
					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeTrue())
				})
			})
		})
	})
})
