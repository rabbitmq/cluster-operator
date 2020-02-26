package status

import (
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func AllReplicasReadyCondition(resources []runtime.Object) RabbitmqClusterCondition {
	condition := generateCondition(AllReplicasReady)
	condition.LastTransitionTime = metav1.Time{
		Time: time.Unix(0, 0),
	}
	for index := range resources {
		switch resource := resources[index].(type) {
		case *appsv1.StatefulSet:
			if resource == nil {
				condition.Status = corev1.ConditionUnknown
				condition.Reason = "MissingStatefulSet"
				condition.Message = "Could not find StatefulSet"

				return condition
			}

			if resource.Status.Replicas == resource.Status.ReadyReplicas {
				condition.Status = corev1.ConditionTrue
				condition.Reason = "AllPodsAreReady"
				return condition
			}

			condition.Status = corev1.ConditionFalse
			condition.Reason = "NotAllPodsReady"
			condition.Message = fmt.Sprintf("%d/%d Pods ready",
				resource.Status.ReadyReplicas,
				resource.Status.Replicas)
		}
	}

	return condition
}
