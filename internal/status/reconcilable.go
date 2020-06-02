package status

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ReconciledCondition(status corev1.ConditionStatus, reason, message string) RabbitmqClusterCondition {
	condition := generateCondition(Reconciled)
	condition.Status = status
	condition.Reason = reason
	condition.Message = message
	condition.LastTransitionTime = metav1.Time{Time: time.Now()}
	return condition
}
