package controllers

import (
	"context"
	"fmt"
	"time"

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

	rabbitmqCluster.Spec.Image = r.DefaultRabbitmqImage
	if requeue, err := r.updateRabbitmqCluster(ctx, rabbitmqCluster, "image"); err != nil {
		return requeue, err
	}

	if rabbitmqCluster.VaultEnabled() {
		rabbitmqCluster.Spec.SecretBackend.Vault.DefaultUserUpdaterImage = &r.DefaultUserUpdaterImage
		if requeue, err := r.updateRabbitmqCluster(ctx, rabbitmqCluster, "default user image"); err != nil {
			return requeue, err
		}
	}

	return 0, nil
}

// updateRabbitmqCluster updates a RabbitmqCluster with the given definition
// it returns a 2 seconds requeue request if update failed due to conflict error
func (r *RabbitmqClusterReconciler) updateRabbitmqCluster(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster, updateType string) (time.Duration, error) {
	logger := ctrl.LoggerFrom(ctx)
	if err := r.Update(ctx, rabbitmqCluster); err != nil {
		if k8serrors.IsConflict(err) {
			logger.Info(fmt.Sprintf("failed to update %s because of conflict; requeueing...", updateType),
				"namespace", rabbitmqCluster.Namespace,
				"name", rabbitmqCluster.Name)
			return 2 * time.Second, nil
		}
		return 0, err
	}
	return 0, nil
}
