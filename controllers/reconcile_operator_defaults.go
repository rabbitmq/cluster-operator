package controllers

import (
	"context"
	"fmt"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"strings"
	"time"
)

// reconcileOperatorDefaults updates current rabbitmqCluster with operator defaults from the Reconciler
// it handles RabbitMQ image, imagePullSecrets, and user updater image
func (r *RabbitmqClusterReconciler) reconcileOperatorDefaults(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) (time.Duration, error) {
	if rabbitmqCluster.Spec.Image == "" {
		rabbitmqCluster.Spec.Image = r.DefaultRabbitmqImage
		if requeue, err := r.updateRabbitmqCluster(ctx, rabbitmqCluster, "image"); err != nil {
			return requeue, err
		}
	}

	if rabbitmqCluster.Spec.ImagePullSecrets == nil {
		// split the comma separated list of default image pull secrets from
		// the 'DEFAULT_IMAGE_PULL_SECRETS' env var, but ignore empty strings.
		for _, reference := range strings.Split(r.DefaultImagePullSecrets, ",") {
			if len(reference) > 0 {
				rabbitmqCluster.Spec.ImagePullSecrets = append(rabbitmqCluster.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: reference})
			}
		}
		if requeue, err := r.updateRabbitmqCluster(ctx, rabbitmqCluster, "image pull secrets"); err != nil {
			return requeue, err
		}
	}

	if rabbitmqCluster.UsesDefaultUserUpdaterImage() {
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
