package controllers

import (
	"context"
	"fmt"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/v2/internal/scaling"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *RabbitmqClusterReconciler) reconcilePVC(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster, desiredSts *appsv1.StatefulSet) error {
	logger := ctrl.LoggerFrom(ctx)
	desiredCapacity, err := persistenceStorageCapacity(desiredSts.Spec.VolumeClaimTemplates)
	if err != nil {
		msg := fmt.Sprintf("Failed to determine PVC capacity: %s", err.Error())
		logger.Error(err, msg)
		r.Recorder.Event(rmq, corev1.EventTypeWarning, "FailedReconcilePersistence", msg)
		return err
	}
	err = scaling.NewPersistenceScaler(r.Clientset).Scale(ctx, *rmq, desiredCapacity)
	if err != nil {
		msg := fmt.Sprintf("Failed to scale PVCs: %s", err.Error())
		logger.Error(fmt.Errorf("hit an error while scaling PVC capacity: %w", err), msg)
		r.Recorder.Event(rmq, corev1.EventTypeWarning, "FailedReconcilePersistence", msg)
	}
	return err
}

func persistenceStorageCapacity(templates []corev1.PersistentVolumeClaim) (k8sresource.Quantity, error) {
	for _, t := range templates {
		if t.Name == "persistence" {
			storage := t.Spec.Resources.Requests[corev1.ResourceStorage]
			if storage.IsZero() {
				return storage, fmt.Errorf(
					"PVC template 'persistence' has spec.resources.requests.storage=0 (or missing). " +
						"If using override.statefulSet.spec.volumeClaimTemplates, you must provide " +
						"the COMPLETE template including spec.resources.requests.storage. " +
						"Overrides replace the entire volumeClaimTemplate, not merge with it")
			}
			return storage, nil
		}
	}
	// No persistence template found - this is valid for ephemeral storage (storage: "0Gi")
	return k8sresource.MustParse("0"), nil
}
