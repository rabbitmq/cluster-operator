// +kubebuilder:object:generate=true
// +groupName=rabbitmq.pivotal.io
package status

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AllReplicasReady RabbitmqClusterConditionType = "AllReplicasReady"
	ClusterAvailable RabbitmqClusterConditionType = "ClusterAvailable"
)

type RabbitmqClusterConditionType string

type RabbitmqClusterCondition struct {
	Type               RabbitmqClusterConditionType `json:"type"`
	Status             corev1.ConditionStatus       `json:"status"`
	LastTransitionTime metav1.Time                  `json:"lastTransitionTime,omitempty"`
	Reason             string                       `json:"reason,omitempty"`
	Message            string                       `json:"message,omitempty"`
}

func generateCondition(conditionType RabbitmqClusterConditionType) RabbitmqClusterCondition {
	return RabbitmqClusterCondition{
		Type:               conditionType,
		Status:             corev1.ConditionUnknown,
		LastTransitionTime: metav1.Time{},
	}
}
