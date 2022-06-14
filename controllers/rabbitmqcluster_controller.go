/*
RabbitMQ Cluster Operator

Copyright 2020 VMware, Inc. All Rights Reserved.

This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"

	"github.com/rabbitmq/cluster-operator/internal/resource"
	"github.com/rabbitmq/cluster-operator/internal/status"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"

	"k8s.io/apimachinery/pkg/types"

	clientretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	apiGVStr = rabbitmqv1beta1.GroupVersion.String()
)

const (
	ownerKey                 = ".metadata.controller"
	ownerKind                = "RabbitmqCluster"
	pauseReconciliationLabel = "rabbitmq.com/pauseReconciliation"
)

// RabbitmqClusterReconciler reconciles a RabbitmqCluster object
type RabbitmqClusterReconciler struct {
	client.Client
	Scheme                  *runtime.Scheme
	Namespace               string
	Recorder                record.EventRecorder
	ClusterConfig           *rest.Config
	Clientset               *kubernetes.Clientset
	PodExecutor             PodExecutor
	DefaultRabbitmqImage    string
	DefaultUserUpdaterImage string
	DefaultImagePullSecrets string
}

// the rbac rule requires an empty row at the end to render
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
// +kubebuilder:rbac:groups="",resources=pods,verbs=update;get;list;watch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;watch;list
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters/status,verbs=get;update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=get;create;patch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update

func (r *RabbitmqClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	rabbitmqCluster, err := r.getRabbitmqCluster(ctx, req.NamespacedName)

	if client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	} else if k8serrors.IsNotFound(err) {
		// No need to requeue if the resource no longer exists
		return ctrl.Result{}, nil
	}

	// Check if the resource has been marked for deletion
	if !rabbitmqCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting")
		return ctrl.Result{}, r.prepareForDeletion(ctx, rabbitmqCluster)
	}

	// exit if pause reconciliation label is set to true
	if v, ok := rabbitmqCluster.Labels[pauseReconciliationLabel]; ok && v == "true" {
		logger.Info("Not reconciling RabbitmqCluster")
		r.Recorder.Event(rabbitmqCluster, corev1.EventTypeWarning,
			"PausedReconciliation", fmt.Sprintf("label '%s' is set to true", pauseReconciliationLabel))

		rabbitmqCluster.Status.SetCondition(status.NoWarnings, corev1.ConditionFalse, "reconciliation paused")
		if writerErr := r.Status().Update(ctx, rabbitmqCluster); writerErr != nil {
			logger.Error(writerErr, "Error trying to Update NoWarnings condition state")
		}
		return ctrl.Result{}, nil
	}

	if requeueAfter, err := r.reconcileOperatorDefaults(ctx, rabbitmqCluster); err != nil || requeueAfter > 0 {
		return ctrl.Result{RequeueAfter: requeueAfter}, err
	}

	// Ensure the resource have a deletion marker
	if err := r.addFinalizerIfNeeded(ctx, rabbitmqCluster); err != nil {
		return ctrl.Result{}, err
	}

	if requeueAfter, err := r.updateStatusConditions(ctx, rabbitmqCluster); err != nil || requeueAfter > 0 {
		return ctrl.Result{RequeueAfter: requeueAfter}, err
	}

	tlsErr := r.reconcileTLS(ctx, rabbitmqCluster)
	if errors.Is(tlsErr, disableNonTLSConfigErr) {
		return ctrl.Result{}, nil
	} else if tlsErr != nil {
		return ctrl.Result{}, tlsErr
	}

	sts, err := r.statefulSet(ctx, rabbitmqCluster)
	// The StatefulSet may not have been created by this point, so ignore Not Found errors
	if client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}
	if sts != nil && statefulSetNeedsQueueRebalance(sts, rabbitmqCluster) {
		if err := r.markForQueueRebalance(ctx, rabbitmqCluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	instanceSpec, err := json.Marshal(rabbitmqCluster.Spec)
	if err != nil {
		logger.Error(err, "Failed to marshal cluster spec")
	}

	logger.Info("Start reconciling",
		"spec", string(instanceSpec))

	resourceBuilder := resource.RabbitmqResourceBuilder{
		Instance: rabbitmqCluster,
		Scheme:   r.Scheme,
	}

	builders := resourceBuilder.ResourceBuilders()

	for _, builder := range builders {
		resource, err := builder.Build()
		if err != nil {
			return ctrl.Result{}, err
		}

		// only StatefulSetBuilder returns true
		if builder.UpdateMayRequireStsRecreate() {
			sts := resource.(*appsv1.StatefulSet)

			current, err := r.statefulSet(ctx, rabbitmqCluster)
			if client.IgnoreNotFound(err) != nil {
				return ctrl.Result{}, err
			}

			// only checks for scale down if statefulSet is created
			// else continue to CreateOrUpdate()
			if !k8serrors.IsNotFound(err) {
				if err := builder.Update(sts); err != nil {
					return ctrl.Result{}, err
				}
				if r.scaleDown(ctx, rabbitmqCluster, current, sts) {
					// return when cluster scale down detected; unsupported operation
					return ctrl.Result{}, nil
				}
			}

			// The PVCs for the StatefulSet may require expanding
			if err = r.reconcilePVC(ctx, rabbitmqCluster, sts); err != nil {
				r.setReconcileSuccess(ctx, rabbitmqCluster, corev1.ConditionFalse, "FailedReconcilePVC", err.Error())
				return ctrl.Result{}, err
			}
		}

		var operationResult controllerutil.OperationResult
		err = clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			var apiError error
			operationResult, apiError = controllerutil.CreateOrUpdate(ctx, r.Client, resource, func() error {
				return builder.Update(resource)
			})
			return apiError
		})
		r.logAndRecordOperationResult(logger, rabbitmqCluster, resource, operationResult, err)
		if err != nil {
			r.setReconcileSuccess(ctx, rabbitmqCluster, corev1.ConditionFalse, "Error", err.Error())
			return ctrl.Result{}, err
		}

		if err = r.annotateIfNeeded(ctx, logger, builder, operationResult, rabbitmqCluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	if requeueAfter, err := r.restartStatefulSetIfNeeded(ctx, logger, rabbitmqCluster); err != nil || requeueAfter > 0 {
		return ctrl.Result{RequeueAfter: requeueAfter}, err
	}

	if err := r.reconcileStatus(ctx, rabbitmqCluster); err != nil {
		return ctrl.Result{}, err
	}

	// By this point the StatefulSet may have finished deploying. Run any
	// post-deploy steps if so, or requeue until the deployment is finished.
	if requeueAfter, err := r.runRabbitmqCLICommandsIfAnnotated(ctx, rabbitmqCluster); err != nil || requeueAfter > 0 {
		if err != nil {
			r.setReconcileSuccess(ctx, rabbitmqCluster, corev1.ConditionFalse, "FailedCLICommand", err.Error())
		}
		return ctrl.Result{RequeueAfter: requeueAfter}, err
	}

	// Set ReconcileSuccess to true and update observedGeneration after all reconciliation steps have finished with no error
	rabbitmqCluster.Status.ObservedGeneration = rabbitmqCluster.GetGeneration()
	r.setReconcileSuccess(ctx, rabbitmqCluster, corev1.ConditionTrue, "Success", "Finish reconciling")

	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *RabbitmqClusterReconciler) getRabbitmqCluster(ctx context.Context, namespacedName types.NamespacedName) (*rabbitmqv1beta1.RabbitmqCluster, error) {
	rabbitmqClusterInstance := &rabbitmqv1beta1.RabbitmqCluster{}
	err := r.Get(ctx, namespacedName, rabbitmqClusterInstance)
	return rabbitmqClusterInstance, err
}

// logAndRecordOperationResult - helper function to log and record events with message and error
// it logs and records 'updated' and 'created' OperationResult, and ignores OperationResult 'unchanged'
func (r *RabbitmqClusterReconciler) logAndRecordOperationResult(logger logr.Logger, rmq runtime.Object, resource runtime.Object, operationResult controllerutil.OperationResult, err error) {
	if operationResult == controllerutil.OperationResultNone && err == nil {
		return
	}

	var operation string
	if operationResult == controllerutil.OperationResultCreated {
		operation = "create"
	}
	if operationResult == controllerutil.OperationResultUpdated {
		operation = "update"
	}

	if err == nil {
		msg := fmt.Sprintf("%sd resource %s of Type %T", operation, resource.(metav1.Object).GetName(), resource.(metav1.Object))
		logger.Info(msg)
		r.Recorder.Event(rmq, corev1.EventTypeNormal, fmt.Sprintf("Successful%s", strings.Title(operation)), msg)
	}

	if err != nil {
		msg := fmt.Sprintf("failed to %s resource %s of Type %T", operation, resource.(metav1.Object).GetName(), resource.(metav1.Object))
		logger.Error(err, msg)
		r.Recorder.Event(rmq, corev1.EventTypeWarning, fmt.Sprintf("Failed%s", strings.Title(operation)), msg)
	}
}

func (r *RabbitmqClusterReconciler) updateStatusConditions(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) (time.Duration, error) {
	logger := ctrl.LoggerFrom(ctx)
	childResources, err := r.getChildResources(ctx, rmq)
	if err != nil {
		return 0, err
	}

	oldConditions := make([]status.RabbitmqClusterCondition, len(rmq.Status.Conditions))
	copy(oldConditions, rmq.Status.Conditions)
	rmq.Status.SetConditions(childResources)

	if !reflect.DeepEqual(rmq.Status.Conditions, oldConditions) {
		if err = r.Status().Update(ctx, rmq); err != nil {
			if k8serrors.IsConflict(err) {
				logger.Info("failed to update status because of conflict; requeueing...",
					"namespace", rmq.Namespace,
					"name", rmq.Name)
				return 2 * time.Second, nil
			}
			return 0, err
		}
	}
	return 0, nil
}

func (r *RabbitmqClusterReconciler) getChildResources(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) ([]runtime.Object, error) {
	sts := &appsv1.StatefulSet{}
	endPoints := &corev1.Endpoints{}

	if err := r.Client.Get(ctx,
		types.NamespacedName{Name: rmq.ChildResourceName("server"), Namespace: rmq.Namespace},
		sts); err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	} else if k8serrors.IsNotFound(err) {
		sts = nil
	}

	if err := r.Client.Get(ctx,
		types.NamespacedName{Name: rmq.ChildResourceName(resource.ServiceSuffix), Namespace: rmq.Namespace},
		endPoints); err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	} else if k8serrors.IsNotFound(err) {
		endPoints = nil
	}

	return []runtime.Object{sts, endPoints}, nil
}

func (r *RabbitmqClusterReconciler) setReconcileSuccess(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster, condition corev1.ConditionStatus, reason, msg string) {
	rabbitmqCluster.Status.SetCondition(status.ReconcileSuccess, condition, reason, msg)
	if writerErr := r.Status().Update(ctx, rabbitmqCluster); writerErr != nil {
		ctrl.LoggerFrom(ctx).Error(writerErr, "Failed to update Custom Resource status",
			"namespace", rabbitmqCluster.Namespace,
			"name", rabbitmqCluster.Name)
	}
}

func (r *RabbitmqClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	for _, resource := range []client.Object{&appsv1.StatefulSet{}, &corev1.ConfigMap{}, &corev1.Service{}} {
		if err := mgr.GetFieldIndexer().IndexField(context.Background(), resource, ownerKey, addResourceToIndex); err != nil {
			return err
		}
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&rabbitmqv1beta1.RabbitmqCluster{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

func addResourceToIndex(rawObj client.Object) []string {
	switch resourceObject := rawObj.(type) {
	case *appsv1.StatefulSet, *corev1.ConfigMap, *corev1.Service, *rbacv1.Role, *rbacv1.RoleBinding, *corev1.ServiceAccount, *corev1.Secret:
		owner := metav1.GetControllerOf(resourceObject)
		return validateAndGetOwner(owner)
	default:
		return nil
	}
}

func validateAndGetOwner(owner *metav1.OwnerReference) []string {
	if owner == nil {
		return nil
	}
	if owner.APIVersion != apiGVStr || owner.Kind != ownerKind {
		return nil
	}
	return []string{owner.Name}
}

func (r *RabbitmqClusterReconciler) markForQueueRebalance(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) error {
	if rmq.ObjectMeta.Annotations == nil {
		rmq.ObjectMeta.Annotations = make(map[string]string)
	}

	if len(rmq.ObjectMeta.Annotations[queueRebalanceAnnotation]) > 0 {
		return nil
	}

	rmq.ObjectMeta.Annotations[queueRebalanceAnnotation] = time.Now().Format(time.RFC3339)
	if err := r.Update(ctx, rmq); err != nil {
		return err
	}
	return nil
}
