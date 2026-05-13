package controllers

import (
	"context"
	"fmt"
	"slices"
	"time"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/v2/internal/rabbitmqclient"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeprecatedFeatureReconciler reconciles a RabbitmqCluster object to check for deprecated features
type DeprecatedFeatureReconciler struct {
	client.Client
	APIReader             client.Reader
	RabbitmqClientFactory rabbitmqclient.RabbitmqClientFactory
	Interval              time.Duration
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeprecatedFeatureReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("deprecated-feature-controller").
		For(&rabbitmqv1beta1.RabbitmqCluster{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters/status,verbs=get;update;patch

// Reconcile reads that state of the cluster for a RabbitmqCluster object and makes changes based on the state read
// and what is in the RabbitmqCluster.Spec
func (r *DeprecatedFeatureReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx).WithValues("namespace", req.Namespace, "name", req.Name)
	logger.Info("Reconciling deprecated features")

	// Fetch the RabbitmqCluster instance
	rmq := &rabbitmqv1beta1.RabbitmqCluster{}
	if err := r.Get(ctx, req.NamespacedName, rmq); err != nil {
		if k8serrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get RabbitmqCluster: %w", err)
	}

	// Requeue after the configured interval in all cases
	requeueResult := ctrl.Result{RequeueAfter: r.Interval}

	rabbitClient, err := r.RabbitmqClientFactory.GetClientForService(ctx, r.APIReader, rmq)
	if err != nil {
		logger.V(1).Info("Failed to get client for service", "error", err)
		return requeueResult, nil
	}

	deprecatedFeatures, err := rabbitClient.ListDeprecatedFeaturesUsed()
	if err != nil {
		logger.V(1).Info("Failed to get deprecated features from management API", "error", err)
		return requeueResult, nil
	}

	var currentInUseDeprecatedFeatures []string
	for _, feature := range deprecatedFeatures {
		currentInUseDeprecatedFeatures = append(currentInUseDeprecatedFeatures, feature.Name)
	}

	// Only update if the status has changed
	if slices.Equal(rmq.Status.DeprecatedFeaturesUsed, currentInUseDeprecatedFeatures) {
		return requeueResult, nil
	}

	// Update the status
	baseRmq := rmq.DeepCopy() // Capture the base object for patching
	rmq.Status.DeprecatedFeaturesUsed = currentInUseDeprecatedFeatures
	if err := r.Status().Patch(ctx, rmq, client.MergeFrom(baseRmq)); err != nil {
		logger.Error(err, "Failed to update deprecated features status")
		if k8serrors.IsConflict(err) {
			// On conflict, do not return an error (that would trigger immediate reconciliation). Instead,
			// requeue the request and wait for the next reconciliation.
			return requeueResult, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to update deprecated features status: %w", err)
	}

	logger.Info("Updated deprecated features status", "deprecatedFeaturesUsed", currentInUseDeprecatedFeatures)
	return requeueResult, nil
}
