package controllers

import (
	"context"
	"fmt"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/v2/internal/resource"
	"github.com/rabbitmq/cluster-operator/v2/internal/scaling"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *RabbitmqClusterReconciler) reconcilePVC(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster, desiredSts *appsv1.StatefulSet) error {
	logger := ctrl.LoggerFrom(ctx)
	if resource.RetainsPersistentVolumeClaimsOnDelete(desiredSts.Spec.PersistentVolumeClaimRetentionPolicy) {
		if err := r.removeRabbitmqClusterOwnerReferencesFromPVCs(ctx, rmq); err != nil {
			return err
		}
	}

	desiredCapacity := persistenceStorageCapacity(desiredSts.Spec.VolumeClaimTemplates)
	err := scaling.NewPersistenceScaler(r.Clientset).Scale(ctx, *rmq, desiredCapacity)
	if err != nil {
		msg := fmt.Sprintf("Failed to scale PVCs: %s", err.Error())
		logger.Error(fmt.Errorf("hit an error while scaling PVC capacity: %w", err), msg)
		r.Recorder.Event(rmq, corev1.EventTypeWarning, "FailedReconcilePersistence", msg)
	}
	return err
}

func (r *RabbitmqClusterReconciler) removeRabbitmqClusterOwnerReferencesFromPVCs(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) error {
	logger := ctrl.LoggerFrom(ctx)
	for i := range ptr.Deref(rmq.Spec.Replicas, 1) {
		pvc, err := r.Clientset.CoreV1().PersistentVolumeClaims(rmq.Namespace).Get(ctx, rmq.PVCName(int(i)), metav1.GetOptions{})
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to get PVC %s: %w", rmq.PVCName(int(i)), err)
		}
		if err != nil {
			continue
		}

		ownerReferences, changed := resource.RemoveRabbitmqClusterOwnerReferences(pvc.OwnerReferences, rmq)
		if !changed {
			continue
		}

		pvc.OwnerReferences = ownerReferences
		if _, err := r.Clientset.CoreV1().PersistentVolumeClaims(rmq.Namespace).Update(ctx, pvc, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update PVC %s owner references: %w", pvc.Name, err)
		}
		logger.Info("Removed RabbitmqCluster owner reference from PVC", "PersistentVolumeClaim", pvc.Name, "RabbitmqCluster", rmq.Name)
	}
	return nil
}

func persistenceStorageCapacity(templates []corev1.PersistentVolumeClaim) k8sresource.Quantity {
	for _, t := range templates {
		if t.Name == "persistence" {
			return t.Spec.Resources.Requests[corev1.ResourceStorage]
		}
	}
	return k8sresource.MustParse("0")
}
