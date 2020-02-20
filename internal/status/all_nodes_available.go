package status

import (
	"fmt"
	"time"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AllNodesAvailableConditionManager struct {
	condition   rabbitmqv1beta1.RabbitmqClusterCondition
	statefulSet *appsv1.StatefulSet
}

func NewAllNodesAvailableConditionManager(childStatefulSet *appsv1.StatefulSet) AllNodesAvailableConditionManager {
	return AllNodesAvailableConditionManager{
		condition:   generateCondition(rabbitmqv1beta1.AllNodesAvailable),
		statefulSet: childStatefulSet,
	}
}

func (manager *AllNodesAvailableConditionManager) Condition() rabbitmqv1beta1.RabbitmqClusterCondition {
	manager.condition.LastTransitionTime = metav1.Time{
		Time: time.Unix(0, 0),
	}

	if manager.statefulSet == nil {
		manager.condition.Status = corev1.ConditionUnknown
		manager.condition.Reason = "CouldNotAccessStatefulSetStatus"
		manager.condition.Message = "There was an error accessing the StatefulSet"

		return manager.condition
	}

	if manager.statefulSet.Status.Replicas == manager.statefulSet.Status.ReadyReplicas {
		manager.condition.Status = corev1.ConditionTrue
		manager.condition.Reason = "AllPodsAreReady"
		return manager.condition
	}

	manager.condition.Status = corev1.ConditionFalse
	manager.condition.Reason = "NotAllPodsAreReady"
	manager.condition.Message = fmt.Sprintf("%d/%d Pods ready",
		manager.statefulSet.Status.ReadyReplicas,
		manager.statefulSet.Status.Replicas)

	return manager.condition
}
