package status

import (
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func generateCondition(conditionType rabbitmqv1beta1.RabbitmqClusterConditionType) rabbitmqv1beta1.RabbitmqClusterCondition {
	return rabbitmqv1beta1.RabbitmqClusterCondition{
		Type:               conditionType,
		Status:             corev1.ConditionUnknown,
		LastTransitionTime: metav1.Time{},
	}
}
