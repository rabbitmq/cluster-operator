package controllers

import (
	"context"
	"fmt"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/scaling"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *RabbitmqClusterReconciler) reconcilePVC(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster, desiredSts *appsv1.StatefulSet) error {
	logger := ctrl.LoggerFrom(ctx)
	desiredCapacity := persistenceStorageCapacity(desiredSts.Spec.VolumeClaimTemplates)
	err := scaling.NewPersistenceScaler(r.Clientset).Scale(ctx, *rmq, desiredCapacity)
	if err != nil {
		msg := fmt.Sprintf("Failed to scale PVCs: %s", err.Error())
		logger.Error(fmt.Errorf("hit an error while scaling PVC capacity: %w", err), msg)
		r.Recorder.Event(rmq, corev1.EventTypeWarning, "FailedReconcilePersistence", msg)
	}
	return err
}

func persistenceStorageCapacity(templates []corev1.PersistentVolumeClaim) k8sresource.Quantity {
	for _, t := range templates {
		if t.Name == "persistence" {
			return t.Spec.Resources.Requests[corev1.ResourceStorage]
		}
	}
	return k8sresource.MustParse("0")
}
