package status_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqstatus "github.com/pivotal/rabbitmq-for-kubernetes/internal/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("ClusterAvailable", func() {
	var (
		childServiceEndpoints *corev1.Endpoints
		existingCondition     *rabbitmqstatus.RabbitmqClusterCondition
		previousConditionTime time.Time
		currentTimeFn         func() time.Time
	)

	BeforeEach(func() {
		childServiceEndpoints = &corev1.Endpoints{}
		previousConditionTime = time.Date(2020, 2, 2, 8, 0, 0, 0, time.UTC)
		currentTimeFn = func() time.Time {
			return time.Date(2020, 2, 2, 9, 6, 0, 0, time.UTC)
		}
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

		When("at least one service endpoint is published", func() {
			BeforeEach(func() {
				childServiceEndpoints.Subsets = []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{
								IP: "1.2.3.4",
							},
							{
								IP: "5.6.7.8",
							},
						},
					},
				}
			})

			It("returns the expected condition and does not update the transition time", func() {
				condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{&corev1.Pod{}, childServiceEndpoints}, existingCondition, currentTimeFn)
				By("having the correct type", func() {
					var conditionType rabbitmqstatus.RabbitmqClusterConditionType = "ClusterAvailable"
					Expect(condition.Type).To(Equal(conditionType))
				})

				By("having status true and reason message", func() {
					Expect(condition.Status).To(Equal(corev1.ConditionTrue))
					Expect(condition.Reason).To(Equal("AtLeastOneEndpointAvailable"))
				})

				By("not updating the transition time", func() {
					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeTrue())
				})
			})
		})

		When("no service endpoint is published", func() {
			BeforeEach(func() {
				childServiceEndpoints.Subsets = []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{},
					},
				}
			})

			It("returns the expected condition and updates transition time", func() {
				condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{childServiceEndpoints}, existingCondition, currentTimeFn)
				By("having the correct type", func() {
					var conditionType rabbitmqstatus.RabbitmqClusterConditionType = "ClusterAvailable"
					Expect(condition.Type).To(Equal(conditionType))
				})

				By("having status false and reason message", func() {
					Expect(condition.Status).To(Equal(corev1.ConditionFalse))
					Expect(condition.Reason).To(Equal("NoEndpointsAvailable"))
					Expect(condition.Message).NotTo(BeEmpty())
				})

				By("updating transition time", func() {
					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
				})
			})
		})

		When("service endpoints do not exist", func() {
			BeforeEach(func() {
				childServiceEndpoints = nil
			})

			It("returns the expected condition and updates transition time", func() {
				condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{childServiceEndpoints}, existingCondition, currentTimeFn)
				By("having the correct type", func() {
					var conditionType rabbitmqstatus.RabbitmqClusterConditionType = "ClusterAvailable"
					Expect(condition.Type).To(Equal(conditionType))
				})

				By("having status true and reason message", func() {
					Expect(condition.Status).To(Equal(corev1.ConditionUnknown))
					Expect(condition.Reason).To(Equal("CouldNotRetrieveEndpoints"))
					Expect(condition.Message).NotTo(BeEmpty())
				})

				By("updating transition time", func() {
					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
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

		When("at least one service endpoint is published", func() {
			BeforeEach(func() {
				childServiceEndpoints.Subsets = []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{
								IP: "1.2.3.4",
							},
							{
								IP: "5.6.7.8",
							},
						},
					},
				}
			})

			It("returns the expected condition and updates the transition time", func() {
				condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{&corev1.Pod{}, childServiceEndpoints}, existingCondition, currentTimeFn)
				By("having the correct type", func() {
					var conditionType rabbitmqstatus.RabbitmqClusterConditionType = "ClusterAvailable"
					Expect(condition.Type).To(Equal(conditionType))
				})

				By("having status true and reason message", func() {
					Expect(condition.Status).To(Equal(corev1.ConditionTrue))
					Expect(condition.Reason).To(Equal("AtLeastOneEndpointAvailable"))
				})

				By("updating the transition time", func() {
					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
				})
			})
		})

		When("no service endpoint is published", func() {
			BeforeEach(func() {
				childServiceEndpoints.Subsets = []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{},
					},
				}
			})

			It("returns the expected condition and does not update transition time", func() {
				condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{childServiceEndpoints}, existingCondition, currentTimeFn)
				By("having the correct type", func() {
					var conditionType rabbitmqstatus.RabbitmqClusterConditionType = "ClusterAvailable"
					Expect(condition.Type).To(Equal(conditionType))
				})

				By("having status false and reason message", func() {
					Expect(condition.Status).To(Equal(corev1.ConditionFalse))
					Expect(condition.Reason).To(Equal("NoEndpointsAvailable"))
					Expect(condition.Message).NotTo(BeEmpty())
				})

				By("not updating transition time", func() {
					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeTrue())
				})
			})
		})

		When("service endpoints do not exist", func() {
			BeforeEach(func() {
				childServiceEndpoints = nil
			})

			It("returns the expected condition and updates transition time", func() {
				condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{childServiceEndpoints}, existingCondition, currentTimeFn)
				By("having the correct type", func() {
					var conditionType rabbitmqstatus.RabbitmqClusterConditionType = "ClusterAvailable"
					Expect(condition.Type).To(Equal(conditionType))
				})

				By("having status true and reason message", func() {
					Expect(condition.Status).To(Equal(corev1.ConditionUnknown))
					Expect(condition.Reason).To(Equal("CouldNotRetrieveEndpoints"))
					Expect(condition.Message).NotTo(BeEmpty())
				})

				By("updating transition time", func() {
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

		When("at least one service endpoint is published", func() {
			BeforeEach(func() {
				childServiceEndpoints.Subsets = []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{
								IP: "1.2.3.4",
							},
							{
								IP: "5.6.7.8",
							},
						},
					},
				}
			})

			It("returns the expected condition and updates the transition time", func() {
				condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{&corev1.Pod{}, childServiceEndpoints}, existingCondition, currentTimeFn)
				By("having the correct type", func() {
					var conditionType rabbitmqstatus.RabbitmqClusterConditionType = "ClusterAvailable"
					Expect(condition.Type).To(Equal(conditionType))
				})

				By("having status true and reason message", func() {
					Expect(condition.Status).To(Equal(corev1.ConditionTrue))
					Expect(condition.Reason).To(Equal("AtLeastOneEndpointAvailable"))
				})

				By("updating the transition time", func() {
					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
				})
			})
		})

		When("no service endpoint is published", func() {
			BeforeEach(func() {
				childServiceEndpoints.Subsets = []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{},
					},
				}
			})

			It("returns the expected condition and updates transition time", func() {
				condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{childServiceEndpoints}, existingCondition, currentTimeFn)
				By("having the correct type", func() {
					var conditionType rabbitmqstatus.RabbitmqClusterConditionType = "ClusterAvailable"
					Expect(condition.Type).To(Equal(conditionType))
				})

				By("having status false and reason message", func() {
					Expect(condition.Status).To(Equal(corev1.ConditionFalse))
					Expect(condition.Reason).To(Equal("NoEndpointsAvailable"))
					Expect(condition.Message).NotTo(BeEmpty())
				})

				By("updating transition time", func() {
					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
				})
			})
		})

		When("service endpoints do not exist", func() {
			BeforeEach(func() {
				childServiceEndpoints = nil
			})

			It("returns the expected condition and does not update transition time", func() {
				condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{childServiceEndpoints}, existingCondition, currentTimeFn)
				By("having the correct type", func() {
					var conditionType rabbitmqstatus.RabbitmqClusterConditionType = "ClusterAvailable"
					Expect(condition.Type).To(Equal(conditionType))
				})

				By("having status true and reason message", func() {
					Expect(condition.Status).To(Equal(corev1.ConditionUnknown))
					Expect(condition.Reason).To(Equal("CouldNotRetrieveEndpoints"))
					Expect(condition.Message).NotTo(BeEmpty())
				})

				By("not updating transition time", func() {
					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeTrue())
				})
			})
		})
	})

	Context("previous condition was not set", func() {
		var expectedTime metav1.Time

		BeforeEach(func() {
			existingCondition = nil
			expectedTime = metav1.Time{Time: time.Date(2020, 2, 2, 9, 6, 0, 0, time.UTC)}
		})

		When("at least one service endpoint is published", func() {
			BeforeEach(func() {
				childServiceEndpoints.Subsets = []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{
								IP: "1.2.3.4",
							},
							{
								IP: "5.6.7.8",
							},
						},
					},
				}
			})

			It("returns the expected condition and updates the transition time", func() {
				condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{&corev1.Pod{}, childServiceEndpoints}, existingCondition, currentTimeFn)
				By("having the correct type", func() {
					var conditionType rabbitmqstatus.RabbitmqClusterConditionType = "ClusterAvailable"
					Expect(condition.Type).To(Equal(conditionType))
				})

				By("having status true and reason message", func() {
					Expect(condition.Status).To(Equal(corev1.ConditionTrue))
					Expect(condition.Reason).To(Equal("AtLeastOneEndpointAvailable"))
				})

				By("updating the transition time", func() {
					Expect(condition.LastTransitionTime).To(Equal(expectedTime))
				})
			})
		})

		When("no service endpoint is published", func() {
			BeforeEach(func() {
				childServiceEndpoints.Subsets = []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{},
					},
				}
			})

			It("returns the expected condition and updates transition time", func() {
				condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{childServiceEndpoints}, existingCondition, currentTimeFn)
				By("having the correct type", func() {
					var conditionType rabbitmqstatus.RabbitmqClusterConditionType = "ClusterAvailable"
					Expect(condition.Type).To(Equal(conditionType))
				})

				By("having status false and reason message", func() {
					Expect(condition.Status).To(Equal(corev1.ConditionFalse))
					Expect(condition.Reason).To(Equal("NoEndpointsAvailable"))
					Expect(condition.Message).NotTo(BeEmpty())
				})

				By("updating the transition time", func() {
					Expect(condition.LastTransitionTime).To(Equal(expectedTime))
				})
			})
		})

		When("service endpoints do not exist", func() {
			BeforeEach(func() {
				childServiceEndpoints = nil
			})

			It("returns the expected condition and updates transition time", func() {
				condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{childServiceEndpoints}, existingCondition, currentTimeFn)
				By("having the correct type", func() {
					var conditionType rabbitmqstatus.RabbitmqClusterConditionType = "ClusterAvailable"
					Expect(condition.Type).To(Equal(conditionType))
				})

				By("having status true and reason message", func() {
					Expect(condition.Status).To(Equal(corev1.ConditionUnknown))
					Expect(condition.Reason).To(Equal("CouldNotRetrieveEndpoints"))
					Expect(condition.Message).NotTo(BeEmpty())
				})

				By("updating the transition time", func() {
					Expect(condition.LastTransitionTime).To(Equal(expectedTime))
				})
			})
		})
	})

})
