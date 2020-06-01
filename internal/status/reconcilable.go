package status

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ReconcilableCondition(status corev1.ConditionStatus, reason string,
	previousCondition ...*RabbitmqClusterCondition) RabbitmqClusterCondition {
	condition := generateCondition(Reconcilable)
	condition.Status = status
	condition.Reason = reason

	if len(previousCondition) == 0 {
		condition.LastTransitionTime = metav1.Time{Time: time.Now()}
		return condition
	}

	if previousCondition[0].Status != condition.Status {
		condition.LastTransitionTime = metav1.Time{Time: time.Now()}
	} else {
		condition.LastTransitionTime = previousCondition[0].LastTransitionTime
	}

	return condition
}
