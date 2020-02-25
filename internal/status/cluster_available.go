package status

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterAvailableConditionManager struct {
	condition RabbitmqClusterCondition
	endpoints *corev1.Endpoints
}

func ClusterAvailableCondition(endpoints *corev1.Endpoints) RabbitmqClusterCondition {
	condition := generateCondition(ClusterAvailable)
	condition.LastTransitionTime = metav1.Time{
		Time: time.Unix(0, 0),
	}

	if endpoints == nil {
		condition.Status = corev1.ConditionFalse
		condition.Reason = "CouldNotAccessServiceEndpoints"
		condition.Message = "Could not verify available service endpoints"
		return condition
	}

	for _, subset := range endpoints.Subsets {
		if len(subset.Addresses) > 0 {
			condition.Status = corev1.ConditionTrue
			return condition
		}
	}

	condition.Status = corev1.ConditionFalse
	condition.Reason = "NoEndpointsAvailable"
	condition.Message = "The ingress service has no endpoints available"
	return condition
}
