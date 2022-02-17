package controllers

import (
	"context"
	"errors"
	"fmt"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// cluster scale down not supported
// log error, publish warning event, and set ReconcileSuccess to false when scale down request detected
func (r *RabbitmqClusterReconciler) scaleDown(ctx context.Context, cluster *v1beta1.RabbitmqCluster, current, sts *appsv1.StatefulSet) bool {
	logger := ctrl.LoggerFrom(ctx)

	currentReplicas := *current.Spec.Replicas
	desiredReplicas := *sts.Spec.Replicas
	if currentReplicas > desiredReplicas {
		msg := fmt.Sprintf("Cluster Scale down not supported; tried to scale cluster from %d nodes to %d nodes", currentReplicas, desiredReplicas)
		reason := "UnsupportedOperation"
		logger.Error(errors.New(reason), msg)
		r.Recorder.Event(cluster, corev1.EventTypeWarning, reason, msg)
		cluster.Status.SetCondition(status.ReconcileSuccess, corev1.ConditionFalse, reason, msg)
		if statusErr := r.Status().Update(ctx, cluster); statusErr != nil {
			logger.Error(statusErr, "Failed to update ReconcileSuccess condition state")
		}
		return true
	}
	return false
}
