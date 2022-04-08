package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rabbitmq/cluster-operator/internal/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("RabbitmqClusterStatus", func() {
	It("sets conditions based on inputs", func() {
		rabbitmqClusterStatus := RabbitmqClusterStatus{}
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

		sts.Status = appsv1.StatefulSetStatus{
			ObservedGeneration: 0,
			Replicas:           0,
			ReadyReplicas:      3,
			CurrentReplicas:    0,
			UpdatedReplicas:    0,
			CurrentRevision:    "",
			UpdateRevision:     "",
			CollisionCount:     nil,
			Conditions:         nil,
		}

		endPoints := &corev1.Endpoints{
			Subsets: []corev1.EndpointSubset{
				{
					Addresses: []corev1.EndpointAddress{
						{
							IP: "127.0.0.1",
						},
					},
				},
			},
		}

		rabbitmqClusterStatus.SetConditions([]runtime.Object{sts, endPoints})

		Expect(rabbitmqClusterStatus.Conditions).To(HaveLen(4))
		Expect(rabbitmqClusterStatus.Conditions[0].Type).To(Equal(status.AllReplicasReady))
		Expect(rabbitmqClusterStatus.Conditions[1].Type).To(Equal(status.ClusterAvailable))
		Expect(rabbitmqClusterStatus.Conditions[2].Type).To(Equal(status.NoWarnings))
		Expect(rabbitmqClusterStatus.Conditions[3].Type).To(Equal(status.ReconcileSuccess))
	})

	It("updates an arbitrary condition", func() {
		someCondition := status.RabbitmqClusterCondition{}
		someCondition.Type = "a-type"
		someCondition.Reason = "whynot"
		someCondition.Status = "perhaps"
		someCondition.LastTransitionTime = metav1.Unix(10, 0)
		rmqStatus := RabbitmqClusterStatus{
			Conditions: []status.RabbitmqClusterCondition{someCondition},
		}

		rmqStatus.SetCondition("a-type",
			corev1.ConditionTrue, "some-reason", "my-message")

		updatedCondition := rmqStatus.Conditions[0]
		Expect(updatedCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(updatedCondition.Reason).To(Equal("some-reason"))
		Expect(updatedCondition.Message).To(Equal("my-message"))

		notExpectedTime := metav1.Unix(10, 0)
		Expect(updatedCondition.LastTransitionTime).NotTo(Equal(notExpectedTime))
		Expect(updatedCondition.LastTransitionTime.Before(&notExpectedTime)).To(BeFalse())
	})
})
