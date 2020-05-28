package status

import (
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func NoWarningsCondition(resources []runtime.Object, existingCondition *ClusterCondition) ClusterCondition {
	condition := generateCondition(NoWarnings)
	if existingCondition != nil {
		condition.LastTransitionTime = existingCondition.LastTransitionTime
	}

	for index := range resources {
		switch resource := resources[index].(type) {
		case *appsv1.StatefulSet:
			if resource == nil {
				condition.Status = corev1.ConditionUnknown
				condition.Reason = "MissingStatefulSet"
				condition.Message = "Could not find StatefulSet"
				goto assignLastTransitionTime
			}

			if !equality.Semantic.DeepEqual(resource.Spec.Template.Spec.Containers[0].Resources.Limits["memory"], resource.Spec.Template.Spec.Containers[0].Resources.Requests["memory"]) {
				condition.Status = corev1.ConditionFalse
				condition.Reason = "MemoryRequestAndLimitDifferent"
				condition.Message = "RabbitMQ container memory resource request and limit must be equal"
				goto assignLastTransitionTime
			}

			condition.Status = corev1.ConditionTrue
			condition.Reason = "NoWarnings"
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
