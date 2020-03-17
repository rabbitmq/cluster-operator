package status_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqstatus "github.com/pivotal/rabbitmq-for-kubernetes/internal/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
		condition := rabbitmqstatus.NoWarningsCondition([]runtime.Object{sts})
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
		sts := &appsv1.StatefulSet{
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

		condition := rabbitmqstatus.NoWarningsCondition([]runtime.Object{sts})
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
	It("is false if the RabbitMQ memory alarm is triggered", func() {
		sts := &appsv1.StatefulSet{
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

		condition := rabbitmqstatus.NoWarningsCondition([]runtime.Object{sts})
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
})
