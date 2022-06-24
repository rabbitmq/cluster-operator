package controllers

import (
	"context"
	"fmt"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sort"
	"strings"
	"time"
)

// mergeImagePullSecrets merge ImagePullSecrets and update
func (r *RabbitmqClusterReconciler) mergeImagePullSecrets(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) (time.Duration, bool, error) {
	checkUniqueName := map[string]struct{}{}
	names := []string{}
	for _, s := range rabbitmqCluster.Spec.ImagePullSecrets {
		if strings.TrimSpace(s.Name) == "" {
			continue
		}
		_, ok := checkUniqueName[s.Name]
		if !ok {
			checkUniqueName[s.Name] = struct{}{}
			names = append(names, s.Name)
		}
	}
	// split the comma separated list of default image pull secrets from
	// the 'DEFAULT_IMAGE_PULL_SECRETS' env var, but ignore empty strings.
	for _, nam := range strings.Split(r.DefaultImagePullSecrets, ",") {
		if strings.TrimSpace(nam) == "" {
			continue
		}
		_, ok := checkUniqueName[nam]
		if !ok {
			checkUniqueName[nam] = struct{}{}
			names = append(names, nam)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		return names[i] > names[j]
	})
	mergedImagePullSecrets := []corev1.LocalObjectReference{}
	for _, nam := range names {
		mergedImagePullSecrets = append(mergedImagePullSecrets, corev1.LocalObjectReference{Name: nam})
	}
	isMerged := false
	if len(mergedImagePullSecrets) != len(rabbitmqCluster.Spec.ImagePullSecrets) {
		isMerged = true
	} else {
		isMerged = !reflect.DeepEqual(mergedImagePullSecrets, rabbitmqCluster.Spec.ImagePullSecrets)
	}
	if isMerged {
		rabbitmqCluster.Spec.ImagePullSecrets = mergedImagePullSecrets
		requeue, err := r.updateRabbitmqCluster(ctx, rabbitmqCluster, "image pull secrets")
		return requeue, isMerged, err
	}
	return 0, isMerged, nil
}

// reconcileOperatorDefaults updates current rabbitmqCluster with operator defaults from the Reconciler
// it handles RabbitMQ image, imagePullSecrets, and user updater image
func (r *RabbitmqClusterReconciler) reconcileOperatorDefaults(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) (time.Duration, error) {
	if rabbitmqCluster.Spec.Image == "" {
		rabbitmqCluster.Spec.Image = r.DefaultRabbitmqImage
		if requeue, err := r.updateRabbitmqCluster(ctx, rabbitmqCluster, "image"); err != nil {
			return requeue, err
		}
	}
	requeue, isMerged, err := r.mergeImagePullSecrets(ctx, rabbitmqCluster)
	if isMerged && err != nil {
		return requeue, err
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
