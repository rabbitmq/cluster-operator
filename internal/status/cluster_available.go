package status

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterAvailableConditionManager struct {
	condition ClusterCondition
	endpoints *corev1.Endpoints
}

func ClusterAvailableCondition(resources []runtime.Object,
	existingCondition *ClusterCondition) ClusterCondition {

	condition := generateCondition(ClusterAvailable)
	if existingCondition != nil {
		condition.LastTransitionTime = existingCondition.LastTransitionTime
	}

	for index := range resources {
		switch resource := resources[index].(type) {
		case *corev1.Endpoints:
			if resource == nil {
				condition.Status = corev1.ConditionUnknown
				condition.Reason = "CouldNotRetrieveEndpoints"
				condition.Message = "Could not verify available service endpoints"
				goto assignLastTransitionTime
			}

			for _, subset := range resource.Subsets {
				if len(subset.Addresses) > 0 {
					condition.Status = corev1.ConditionTrue
					condition.Reason = "AtLeastOneEndpointAvailable"
					goto assignLastTransitionTime
				}
			}

			condition.Status = corev1.ConditionFalse
			condition.Reason = "NoEndpointsAvailable"
			condition.Message = "The ingress service has no endpoints available"
		}
	}

assignLastTransitionTime:
	if existingCondition == nil || existingCondition.Status != condition.Status {
		condition.LastTransitionTime = metav1.Time{
			Time: time.Now(),
		}
	}

	return condition
}
