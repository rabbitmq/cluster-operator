package status

import (
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AllNodesAvailableConditionManager struct {
	condition   RabbitmqClusterCondition
	statefulSet *appsv1.StatefulSet
}

func AllNodesAvailableCondition(statefulSet *appsv1.StatefulSet) RabbitmqClusterCondition {
	condition := generateCondition(AllNodesAvailable)
	condition.LastTransitionTime = metav1.Time{
		Time: time.Unix(0, 0),
	}

	if statefulSet == nil {
		condition.Status = corev1.ConditionUnknown
		condition.Reason = "CouldNotAccessStatefulSetStatus"
		condition.Message = "There was an error accessing the StatefulSet"

		return condition
	}

	if statefulSet.Status.Replicas == statefulSet.Status.ReadyReplicas {
		condition.Status = corev1.ConditionTrue
		condition.Reason = "AllPodsAreReady"
		return condition
	}

	condition.Status = corev1.ConditionFalse
	condition.Reason = "NotAllPodsAreReady"
	condition.Message = fmt.Sprintf("%d/%d Pods ready",
		statefulSet.Status.ReadyReplicas,
		statefulSet.Status.Replicas)

	return condition
}
