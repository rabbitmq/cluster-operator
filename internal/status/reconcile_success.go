package status

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ReconcileSuccessCondition(status corev1.ConditionStatus, reason, message string) RabbitmqClusterCondition {
	return RabbitmqClusterCondition{
		Type:               ReconcileSuccess,
		Status:             status,
		LastTransitionTime: metav1.Time{Time: time.Now()},
		Reason:             reason,
		Message:            message,
	}
}
