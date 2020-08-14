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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("NoWarnings", func() {
	It("is true if no resource limits are set", func() {
		sts := &appsv1.StatefulSet{
			Spec: appsv1.StatefulSetSpec{
				Replicas: nil,
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Resources: corev1.ResourceRequirements{},
							},
						},
					},
				},
			},
		}
		condition := rabbitmqstatus.NoWarningsCondition([]runtime.Object{sts}, nil)
		By("having the correct type", func() {
			var conditionType rabbitmqstatus.RabbitmqClusterConditionType = "NoWarnings"
			Expect(condition.Type).To(Equal(conditionType))
		})

		By("having status false and reason message", func() {
			Expect(condition.Status).To(Equal(corev1.ConditionTrue))
			Expect(condition.Reason).To(Equal("NoWarnings"))
			Expect(condition.Message).To(BeEmpty())
		})
	})

	It("is false if the memory request does not match the memory limit", func() {
		condition := rabbitmqstatus.NoWarningsCondition([]runtime.Object{memoryWarningStatefulSet()}, nil)
		By("having the correct type", func() {
			var conditionType rabbitmqstatus.RabbitmqClusterConditionType = "NoWarnings"
			Expect(condition.Type).To(Equal(conditionType))
		})

		By("having status false and reason message", func() {
			Expect(condition.Status).To(Equal(corev1.ConditionFalse))
			Expect(condition.Reason).To(Equal("MemoryRequestAndLimitDifferent"))
			Expect(condition.Message).NotTo(BeEmpty())
		})
	})

	It("is unknown when the StatefulSet does not exist", func() {
		var sts *appsv1.StatefulSet = nil
		condition := rabbitmqstatus.NoWarningsCondition([]runtime.Object{sts}, nil)

		By("having status unknown and reason", func() {
			Expect(condition.Status).To(Equal(corev1.ConditionUnknown))
			Expect(condition.Reason).To(Equal("MissingStatefulSet"))
			Expect(condition.Message).ToNot(BeEmpty())
		})
	})

	Context("condition transitions", func() {
		var (
			previousConditionTime time.Time
			existingCondition     *rabbitmqstatus.RabbitmqClusterCondition
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
				It("does not update transition time", func() {
					condition := rabbitmqstatus.NoWarningsCondition([]runtime.Object{noMemoryWarningStatefulSet()}, existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeTrue())
				})
			})

			When("transitions to false", func() {
				It("updates transition time", func() {
					condition := rabbitmqstatus.NoWarningsCondition([]runtime.Object{memoryWarningStatefulSet()}, existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
				})
			})

			When("transitions to unknown", func() {
				It("updates transition time", func() {
					condition := rabbitmqstatus.NoWarningsCondition([]runtime.Object{nil}, existingCondition)

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
				It("updates transition time", func() {
					condition := rabbitmqstatus.NoWarningsCondition([]runtime.Object{noMemoryWarningStatefulSet()}, existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
				})
			})

			When("remains false", func() {
				It("does not update transition time", func() {
					condition := rabbitmqstatus.NoWarningsCondition([]runtime.Object{memoryWarningStatefulSet()}, existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeTrue())
				})
			})

			When("transitions to unknown", func() {
				It("updates transition time", func() {
					condition := rabbitmqstatus.NoWarningsCondition([]runtime.Object{nil}, existingCondition)

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
				It("updates transition time", func() {
					condition := rabbitmqstatus.NoWarningsCondition([]runtime.Object{memoryWarningStatefulSet()}, existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
				})
			})

			When("transitions to false", func() {

				It("updates transition time", func() {

					condition := rabbitmqstatus.NoWarningsCondition([]runtime.Object{memoryWarningStatefulSet()}, existingCondition)

					Expect(existingCondition).NotTo(BeNil())
					existingConditionTime := existingCondition.LastTransitionTime.DeepCopy()
					Expect(condition.LastTransitionTime.Equal(existingConditionTime)).To(BeFalse())
					Expect(condition.LastTransitionTime.Before(existingConditionTime)).To(BeFalse())
				})
			})

			When("remains unknown", func() {
				It("does not update transition time", func() {
					condition := rabbitmqstatus.NoWarningsCondition([]runtime.Object{nil}, existingCondition)

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

				It("updates transition time", func() {
					condition := rabbitmqstatus.NoWarningsCondition([]runtime.Object{memoryWarningStatefulSet()}, existingCondition)

					Expect(condition.LastTransitionTime).ToNot(Equal(emptyTime))
				})
			})

			When("transitions to false", func() {
				It("updates transition time", func() {
					condition := rabbitmqstatus.NoWarningsCondition([]runtime.Object{memoryWarningStatefulSet()}, existingCondition)

					Expect(condition.LastTransitionTime).ToNot(Equal(emptyTime))
				})
			})

			When("transitions to unknown", func() {
				It("updates transition time", func() {
					condition := rabbitmqstatus.NoWarningsCondition([]runtime.Object{nil}, existingCondition)

					Expect(condition.LastTransitionTime).ToNot(Equal(emptyTime))
				})
			})
		})
	})
})

func memoryWarningStatefulSet() *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: nil,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Limits: map[corev1.ResourceName]resource.Quantity{
									"memory": resource.MustParse("100Mi"),
								},
								Requests: map[corev1.ResourceName]resource.Quantity{
									"memory": resource.MustParse("50Mi"),
								},
							},
						},
					},
				},
			},
		},
	}
}

func noMemoryWarningStatefulSet() *appsv1.StatefulSet {
	sts := &appsv1.StatefulSet{}
	sts.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Resources: corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					"memory": resource.MustParse("100Mi"),
				},
				Requests: map[corev1.ResourceName]resource.Quantity{
					"memory": resource.MustParse("100Mi"),
				},
			},
		},
	}

	return sts
}
