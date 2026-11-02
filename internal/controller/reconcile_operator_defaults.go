package controllers

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
)

// reconcileOperatorDefaults enforces operator-controlled image overrides when ControlRabbitmqImage is enabled.
// Admission-time defaulting (image, imagePullSecrets, default user updater image) is handled by the mutating webhook.
func (r *RabbitmqClusterReconciler) reconcileOperatorDefaults(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) (time.Duration, error) {
	if !r.ControlRabbitmqImage {
		return 0, nil
	}

	base := rabbitmqCluster.DeepCopy()
	changed := false
	if rabbitmqCluster.Spec.Image != r.DefaultRabbitmqImage {
		rabbitmqCluster.Spec.Image = r.DefaultRabbitmqImage
		changed = true
	}
	if rabbitmqCluster.VaultEnabled() {
		image := rabbitmqCluster.Spec.SecretBackend.Vault.DefaultUserUpdaterImage
		if image == nil || *image != r.DefaultUserUpdaterImage {
			image := r.DefaultUserUpdaterImage
			rabbitmqCluster.Spec.SecretBackend.Vault.DefaultUserUpdaterImage = &image
			changed = true
		}
	}
	if changed {
		return r.patchRabbitmqCluster(ctx, rabbitmqCluster, base)
	}

	return 0, nil
}

// patchRabbitmqCluster patches only changed fields with optimistic locking.
// it returns a 2 seconds requeue request if update failed due to conflict error
func (r *RabbitmqClusterReconciler) patchRabbitmqCluster(ctx context.Context, rabbitmqCluster, base *rabbitmqv1beta1.RabbitmqCluster) (time.Duration, error) {
	logger := ctrl.LoggerFrom(ctx)
	if err := r.Patch(ctx, rabbitmqCluster, client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{})); err != nil {
		if k8serrors.IsConflict(err) {
			logger.Info("failed to patch images because of conflict; requeueing...",
				"namespace", rabbitmqCluster.Namespace,
				"name", rabbitmqCluster.Name)
			return 2 * time.Second, nil
		}
		return 0, err
	}
	return 0, nil
}
