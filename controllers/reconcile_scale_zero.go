package controllers

import (
	"context"
	"fmt"
	"strconv"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/v2/internal/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const beforeZeroReplicasConfigured = "rabbitmq.com/before-zero-replicas-configured"

// ScaleToZero checks if the desired replicas is zero and the current replicas is not zero.
func ScaleToZero(current, sts *appsv1.StatefulSet) bool {
	currentReplicas := *current.Spec.Replicas
	desiredReplicas := *sts.Spec.Replicas
	return desiredReplicas == 0 && currentReplicas > 0
}

// ScaleFromZero checks if the current replicas is zero and the desired replicas is greater than zero.
func ScaleFromZero(current, sts *appsv1.StatefulSet) bool {
	currentReplicas := *current.Spec.Replicas
	desiredReplicas := *sts.Spec.Replicas
	return currentReplicas == 0 && desiredReplicas > 0
}

// scaleDownFromZero checks if the current replicas is desired replicas would be greatter than replicas configured before zero state.
func (r *RabbitmqClusterReconciler) scaleFromZeroToBeforeReplicasConfigured(ctx context.Context, cluster *v1beta1.RabbitmqCluster, sts *appsv1.StatefulSet) bool {
	logger := ctrl.LoggerFrom(ctx)
	var err error
	var beforeZeroReplicas int64
	desiredReplicas := *sts.Spec.Replicas
	annotationValue, ok := cluster.Annotations[beforeZeroReplicasConfigured]
	if !ok {
		return false
	}

	beforeZeroReplicas, err = strconv.ParseInt(annotationValue, 10, 32)
	if err != nil {
		msg := "Failed to convert string to integer for before-zero-replicas-configuration annotation"
		reason := "TransformErrorOperation"
		err = r.recordEventsAndSetCondition(ctx, cluster, status.ReconcileSuccess, corev1.ConditionFalse, corev1.EventTypeWarning, reason, msg)
		if err != nil {
			logger.V(1).Info(err.Error())
		}
		return true
	}
	if desiredReplicas != int32(beforeZeroReplicas) {
		msg := fmt.Sprintf("Unsupported operation; when scaling from zero, you can only restore the previous number of replicas (%d)", int32(beforeZeroReplicas))
		reason := "UnsupportedOperation"
		err = r.recordEventsAndSetCondition(ctx, cluster, status.ReconcileSuccess, corev1.ConditionFalse, corev1.EventTypeWarning, reason, msg)
		if err != nil {
			logger.V(1).Info(err.Error())
		}
		return true
	}
	return false

}

// saveReplicasBeforeZero saves the current replicas count in an annotation before scaling down to zero.
// This is used to prevent scaling down when the cluster change from zero replicas to a number less than the saved replicas count.
func (r *RabbitmqClusterReconciler) saveReplicasBeforeZero(ctx context.Context, cluster *v1beta1.RabbitmqCluster, current *appsv1.StatefulSet) error {
	currentReplicas := *current.Spec.Replicas
	logger := ctrl.LoggerFrom(ctx)
	msg := "Cluster Scale down to 0 replicas"
	reason := "ScaleDownToZero"
	logger.Info(msg)
	r.Recorder.Event(cluster, corev1.EventTypeNormal, reason, msg)
	return r.updateAnnotation(ctx, cluster, cluster.Namespace, cluster.Name, beforeZeroReplicasConfigured, fmt.Sprint(currentReplicas))
}

// If the annotation rabbitmq.com/before-zero-replicas-configured exists it will be deleted.
func (r *RabbitmqClusterReconciler) removeReplicasBeforeZeroAnnotationIfExists(ctx context.Context, cluster *v1beta1.RabbitmqCluster) {
	if _, ok := cluster.Annotations[beforeZeroReplicasConfigured]; ok {
		r.deleteAnnotation(ctx, cluster, beforeZeroReplicasConfigured)
	}
}

func (r *RabbitmqClusterReconciler) recordEventsAndSetCondition(ctx context.Context, cluster *v1beta1.RabbitmqCluster, condType status.RabbitmqClusterConditionType, condStatus corev1.ConditionStatus, eventType, reason, msg string) error {
	r.Recorder.Event(cluster, eventType, reason, msg)
	cluster.Status.SetCondition(condType, condStatus, reason, msg)
	return r.Status().Update(ctx, cluster)
}
