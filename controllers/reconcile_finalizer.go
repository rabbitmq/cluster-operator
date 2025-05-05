package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/v2/internal/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	clientretry "k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const deletionFinalizer = "deletion.finalizers.rabbitmqclusters.rabbitmq.com"

// addFinalizerIfNeeded adds a deletion finalizer if the RabbitmqCluster does not have one yet and is not marked for deletion
func (r *RabbitmqClusterReconciler) addFinalizerIfNeeded(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) error {
	if rabbitmqCluster.ObjectMeta.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(rabbitmqCluster, deletionFinalizer) {
		return clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			controllerutil.AddFinalizer(rabbitmqCluster, deletionFinalizer)
			return r.Client.Update(ctx, rabbitmqCluster)
		})
	}
	return nil
}

func (r *RabbitmqClusterReconciler) removeFinalizer(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) error {
	currentRabbitmqCluster := &rabbitmqv1beta1.RabbitmqCluster{}
	currentRabbitmqCluster.Name = rabbitmqCluster.Name
	currentRabbitmqCluster.Namespace = rabbitmqCluster.Namespace

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, currentRabbitmqCluster, func() error {
		controllerutil.RemoveFinalizer(currentRabbitmqCluster, deletionFinalizer)
		return nil
	})

	if err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "Failed to remove finalizer for deletion")
		return client.IgnoreNotFound(err)
	}

	return nil
}

func (r *RabbitmqClusterReconciler) prepareForDeletion(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) error {
	if controllerutil.ContainsFinalizer(rabbitmqCluster, deletionFinalizer) {
		clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			return r.addRabbitmqDeletionLabel(ctx, rabbitmqCluster)
		})

		// wait for up to 3 seconds for the labels to propagate
		timeout := time.Now().Add(3 * time.Second)
		for time.Now().Before(timeout) && !r.checkIfLabelPropagated(ctx, rabbitmqCluster) {
			time.Sleep(200 * time.Millisecond)
		}

		if err := r.removeFinalizer(ctx, rabbitmqCluster); err != nil {
			ctrl.LoggerFrom(ctx).Error(err, "Failed to remove finalizer for deletion")
			return err
		}
	}
	return nil
}

func (r *RabbitmqClusterReconciler) addRabbitmqDeletionLabel(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) error {
	pods := &corev1.PodList{}
	selector, err := labels.Parse(fmt.Sprintf("app.kubernetes.io/name=%s", rabbitmqCluster.Name))
	if err != nil {
		return err
	}
	listOptions := client.ListOptions{
		LabelSelector: selector,
		Namespace:     rabbitmqCluster.Namespace,
	}

	if err := r.List(ctx, pods, &listOptions); err != nil {
		return err
	}

	for i := 0; i < len(pods.Items); i++ {
		pod := &pods.Items[i]
		pod.Labels[resource.DeletionMarker] = "true"
		if err := r.Update(ctx, pod); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("cannot Update Pod %s in Namespace %s: %w", pod.Name, pod.Namespace, err)
		}
	}

	return nil
}

func (r *RabbitmqClusterReconciler) checkIfLabelPropagated(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) bool {
	logger := ctrl.LoggerFrom(ctx)
	podName := fmt.Sprintf("%s-0", rabbitmqCluster.ChildResourceName("server"))
	cmd := "cat /etc/pod-info/skipPreStopChecks"
	stdout, _, err := r.exec(rabbitmqCluster.Namespace, podName, "rabbitmq", "sh", "-c", cmd)
	if err != nil {
		logger.Info("Failed to check for deletion label propagation, deleting anyway", "pod", podName, "command", cmd, "stdout", stdout)
		return true
	}
	return strings.HasPrefix(stdout, "true")
}
