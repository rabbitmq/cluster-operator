// +kubebuilder:object:generate=true
// +groupName=rabbitmq.com
package status

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AllReplicasReady RabbitmqClusterConditionType = "AllReplicasReady"
	ClusterAvailable RabbitmqClusterConditionType = "ClusterAvailable"
	NoWarnings       RabbitmqClusterConditionType = "NoWarnings"
)

type RabbitmqClusterConditionType string

type RabbitmqClusterCondition struct {
	// Type indicates the scope of RabbitmqCluster status addressed by the condition.
	Type RabbitmqClusterConditionType `json:"type"`
	// True, False, or Unknown
	Status corev1.ConditionStatus `json:"status"`
	// The last time this Condition type changed.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// One word, camel-case reason for current status of the condition.
	Reason string `json:"reason,omitempty"`
	// Full text reason for current status of the condition.
	Message string `json:"message,omitempty"`
}

func generateCondition(conditionType RabbitmqClusterConditionType) RabbitmqClusterCondition {
	return RabbitmqClusterCondition{
		Type:               conditionType,
		Status:             corev1.ConditionUnknown,
		LastTransitionTime: metav1.Time{},
	}
}
