// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
//

package status_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqstatus "github.com/rabbitmq/cluster-operator/internal/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("ClusterAvailable", func() {
	var (
		childServiceEndpoints *corev1.Endpoints
		existingCondition     *rabbitmqstatus.RabbitmqClusterCondition
	)

	BeforeEach(func() {
		childServiceEndpoints = &corev1.Endpoints{}
	})

	Context("condition status and reason", func() {
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

			It("returns a condition with state true", func() {
				condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{&corev1.Pod{}, childServiceEndpoints}, existingCondition)

				Expect(condition.Status).To(Equal(corev1.ConditionTrue))
				Expect(condition.Reason).To(Equal("AtLeastOneEndpointAvailable"))
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

			It("returns a condition with state false", func() {
				condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{childServiceEndpoints}, existingCondition)

				Expect(condition.Status).To(Equal(corev1.ConditionFalse))
				Expect(condition.Reason).To(Equal("NoEndpointsAvailable"))
				Expect(condition.Message).NotTo(BeEmpty())
			})
		})

		When("service endpoints do not exist", func() {
			BeforeEach(func() {
				childServiceEndpoints = nil
			})

			It("returns a condition with state unknown", func() {
				condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{childServiceEndpoints}, existingCondition)
				Expect(condition.Status).To(Equal(corev1.ConditionUnknown))
				Expect(condition.Reason).To(Equal("CouldNotRetrieveEndpoints"))
				Expect(condition.Message).NotTo(BeEmpty())
			})
		})
	})

	Context("condition transitions", func() {
		var (
			previousConditionTime time.Time
		)

		BeforeEach(func() {
			previousConditionTime = time.Date(2020, 2, 2, 8, 0, 0, 0, time.UTC)
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

			When("remains true", func() {
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

				It("does not update transition time", func() {
					condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{&corev1.Pod{}, childServiceEndpoints}, existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeTrue())
				})
			})

			When("transitions to false", func() {
				BeforeEach(func() {
					childServiceEndpoints.Subsets = []corev1.EndpointSubset{
						{
							Addresses: []corev1.EndpointAddress{},
						},
					}
				})

				It("updates transition time", func() {
					condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{childServiceEndpoints}, existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
				})
			})

			When("transitions to unknown", func() {
				BeforeEach(func() {
					childServiceEndpoints = nil
				})

				It("updates transition time", func() {
					condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{childServiceEndpoints}, existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
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

			When("transitions to true", func() {
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

				It("updates transition time", func() {
					condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{&corev1.Pod{}, childServiceEndpoints}, existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
				})
			})

			When("remains false", func() {
				BeforeEach(func() {
					childServiceEndpoints.Subsets = []corev1.EndpointSubset{
						{
							Addresses: []corev1.EndpointAddress{},
						},
					}
				})

				It("does not update transition time", func() {
					condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{childServiceEndpoints}, existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeTrue())
				})
			})

			When("transitions	to unknown", func() {
				BeforeEach(func() {
					childServiceEndpoints = nil
				})

				It("updates transition time", func() {
					condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{childServiceEndpoints}, existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
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

			When("transitions to true", func() {
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

				It("updates transition time", func() {
					condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{&corev1.Pod{}, childServiceEndpoints}, existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
				})
			})

			When("transitions to false", func() {
				BeforeEach(func() {
					childServiceEndpoints.Subsets = []corev1.EndpointSubset{
						{
							Addresses: []corev1.EndpointAddress{},
						},
					}
				})

				It("updates transition time", func() {
					condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{childServiceEndpoints}, existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
				})
			})

			When("remains unknown", func() {
				BeforeEach(func() {
					childServiceEndpoints = nil
				})

				It("does not update transition time", func() {
					condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{childServiceEndpoints}, existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeTrue())
				})
			})
		})

		Context("previous condition was not set", func() {
			var emptyTime metav1.Time

			BeforeEach(func() {
				existingCondition = nil
				emptyTime = metav1.Time{}
			})

			When("transitions to true", func() {
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

				It("updates transition time", func() {
					condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{&corev1.Pod{}, childServiceEndpoints}, existingCondition)

					Expect(condition.LastTransitionTime).ToNot(Equal(emptyTime))
				})
			})

			When("transitions to false", func() {
				BeforeEach(func() {
					childServiceEndpoints.Subsets = []corev1.EndpointSubset{
						{
							Addresses: []corev1.EndpointAddress{},
						},
					}
				})

				It("updates transition time", func() {
					condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{childServiceEndpoints}, existingCondition)

					Expect(condition.LastTransitionTime).ToNot(Equal(emptyTime))
				})
			})

			When("transitions to unknown", func() {
				BeforeEach(func() {
					childServiceEndpoints = nil
				})

				It("updates transition time", func() {
					condition := rabbitmqstatus.ClusterAvailableCondition([]runtime.Object{childServiceEndpoints}, existingCondition)

					Expect(condition.LastTransitionTime).ToNot(Equal(emptyTime))
				})
			})
		})
	})

})
