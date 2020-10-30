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
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/rabbitmq/cluster-operator/internal/resource"
	"github.com/rabbitmq/cluster-operator/internal/status"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"

	"k8s.io/apimachinery/pkg/types"

	clientretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/go-logr/logr"
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
	ownerKey  = ".metadata.controller"
	ownerKind = "RabbitmqCluster"
)

// RabbitmqClusterReconciler reconciles a RabbitmqCluster object
type RabbitmqClusterReconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	Namespace     string
	Recorder      record.EventRecorder
	ClusterConfig *rest.Config
	Clientset     *kubernetes.Clientset
	PodExecutor   PodExecutor
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
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update

func (r *RabbitmqClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	logger := r.Log

	rabbitmqCluster, err := r.getRabbitmqCluster(ctx, req.NamespacedName)

	if client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	} else if errors.IsNotFound(err) {
		// No need to requeue if the resource no longer exists
		return ctrl.Result{}, nil
	}

	// Check if the resource has been marked for deletion
	if !rabbitmqCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting RabbitmqCluster",
			"namespace", rabbitmqCluster.Namespace,
			"name", rabbitmqCluster.Name)
		return ctrl.Result{}, r.prepareForDeletion(ctx, rabbitmqCluster)
	}

	// Ensure the resource have a deletion marker
	if err := r.addFinalizerIfNeeded(ctx, rabbitmqCluster); err != nil {
		return ctrl.Result{}, err
	}

	// TLS: check if specified, and if secret exists
	if rabbitmqCluster.TLSEnabled() {
		if result, err := r.checkTLSSecrets(ctx, rabbitmqCluster); err != nil {
			return result, err
		}
	}

	childResources, err := r.getChildResources(ctx, *rabbitmqCluster)
	if err != nil {
		return ctrl.Result{}, err
	}

	oldConditions := make([]status.RabbitmqClusterCondition, len(rabbitmqCluster.Status.Conditions))
	copy(oldConditions, rabbitmqCluster.Status.Conditions)
	rabbitmqCluster.Status.SetConditions(childResources)

	if !reflect.DeepEqual(rabbitmqCluster.Status.Conditions, oldConditions) {
		if err = r.Status().Update(ctx, rabbitmqCluster); err != nil {
			return ctrl.Result{}, err
		}
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

	logger.Info("Start reconciling RabbitmqCluster",
		"namespace", rabbitmqCluster.Namespace,
		"name", rabbitmqCluster.Name,
		"spec", string(instanceSpec))

	resourceBuilder := resource.RabbitmqResourceBuilder{
		Instance: rabbitmqCluster,
		Scheme:   r.Scheme,
	}

	builders, err := resourceBuilder.ResourceBuilders()
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, builder := range builders {
		resource, err := builder.Build()
		if err != nil {
			return ctrl.Result{}, err
		}

		var operationResult controllerutil.OperationResult
		err = clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			var apiError error
			operationResult, apiError = controllerutil.CreateOrUpdate(ctx, r, resource, func() error {
				return builder.Update(resource)
			})
			return apiError
		})
		r.logAndRecordOperationResult(rabbitmqCluster, resource, operationResult, err)
		if err != nil {
			rabbitmqCluster.Status.SetCondition(status.ReconcileSuccess, corev1.ConditionFalse, "Error", err.Error())
			if writerErr := r.Status().Update(ctx, rabbitmqCluster); writerErr != nil {
				r.Log.Error(writerErr, "Error trying to Update ReconcileSuccess condition state",
					"namespace", rabbitmqCluster.Namespace,
					"name", rabbitmqCluster.Name)
			}
			return ctrl.Result{}, err
		}

		if err = r.annotateIfNeeded(ctx, builder, operationResult, rabbitmqCluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	requeueAfter, err := r.restartStatefulSetIfNeeded(ctx, rabbitmqCluster)
	if err != nil {
		return ctrl.Result{}, err
	}
	if requeueAfter > 0 {
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	// Set ReconcileSuccess to true here because all CRUD operations to Kube API related
	// to child resources returned no error
	rabbitmqCluster.Status.SetCondition(status.ReconcileSuccess, corev1.ConditionTrue, "Success", "Created or Updated all child resources")
	if writerErr := r.Status().Update(ctx, rabbitmqCluster); writerErr != nil {
		r.Log.Error(writerErr, "Error trying to Update Custom Resource status",
			"namespace", rabbitmqCluster.Namespace,
			"name", rabbitmqCluster.Name)
	}

	if err := r.setDefaultUserStatus(ctx, rabbitmqCluster); err != nil {
		return ctrl.Result{}, err
	}

	// By this point the StatefulSet may have finished deploying. Run any
	// post-deploy steps if so, or requeue until the deployment is finished.
	requeueAfter, err = r.runRabbitmqCLICommandsIfAnnotated(ctx, rabbitmqCluster)
	if err != nil {
		return ctrl.Result{}, err
	}
	if requeueAfter > 0 {
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	logger.Info("Finished reconciling RabbitmqCluster",
		"namespace", rabbitmqCluster.Namespace,
		"name", rabbitmqCluster.Name)

	return ctrl.Result{}, nil
}

// logAndRecordOperationResult - helper function to log and record events with message and error
// it logs and records 'updated' and 'created' OperationResult, and ignores OperationResult 'unchanged'
func (r *RabbitmqClusterReconciler) logAndRecordOperationResult(rmq runtime.Object, resource runtime.Object, operationResult controllerutil.OperationResult, err error) {
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
		r.Log.Info(msg)
		r.Recorder.Event(rmq, corev1.EventTypeNormal, fmt.Sprintf("Successful%s", strings.Title(operation)), msg)
	}

	if err != nil {
		msg := fmt.Sprintf("failed to %s resource %s of Type %T", operation, resource.(metav1.Object).GetName(), resource.(metav1.Object))
		r.Log.Error(err, msg)
		r.Recorder.Event(rmq, corev1.EventTypeWarning, fmt.Sprintf("Failed%s", strings.Title(operation)), msg)
	}
}

func (r *RabbitmqClusterReconciler) getChildResources(ctx context.Context, rmq rabbitmqv1beta1.RabbitmqCluster) ([]runtime.Object, error) {
	sts := &appsv1.StatefulSet{}
	endPoints := &corev1.Endpoints{}

	if err := r.Client.Get(ctx,
		types.NamespacedName{Name: rmq.ChildResourceName("server"), Namespace: rmq.Namespace},
		sts); err != nil && !errors.IsNotFound(err) {
		return nil, err
	} else if errors.IsNotFound(err) {
		sts = nil
	}

	if err := r.Client.Get(ctx,
		types.NamespacedName{Name: rmq.ChildResourceName("client"), Namespace: rmq.Namespace},
		endPoints); err != nil && !errors.IsNotFound(err) {
		return nil, err
	} else if errors.IsNotFound(err) {
		endPoints = nil
	}

	return []runtime.Object{sts, endPoints}, nil
}

func (r *RabbitmqClusterReconciler) getRabbitmqCluster(ctx context.Context, namespacedName types.NamespacedName) (*rabbitmqv1beta1.RabbitmqCluster, error) {
	rabbitmqClusterInstance := &rabbitmqv1beta1.RabbitmqCluster{}
	err := r.Get(ctx, namespacedName, rabbitmqClusterInstance)
	return rabbitmqClusterInstance, err
}

func (r *RabbitmqClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	for _, resource := range []runtime.Object{&appsv1.StatefulSet{}, &corev1.ConfigMap{}, &corev1.Service{}} {
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

func addResourceToIndex(rawObj runtime.Object) []string {
	switch resourceObject := rawObj.(type) {
	case *appsv1.StatefulSet:
		owner := metav1.GetControllerOf(resourceObject)
		return validateAndGetOwner(owner)
	case *corev1.ConfigMap:
		owner := metav1.GetControllerOf(resourceObject)
		return validateAndGetOwner(owner)
	case *corev1.Service:
		owner := metav1.GetControllerOf(resourceObject)
		return validateAndGetOwner(owner)
	case *rbacv1.Role:
		owner := metav1.GetControllerOf(resourceObject)
		return validateAndGetOwner(owner)
	case *rbacv1.RoleBinding:
		owner := metav1.GetControllerOf(resourceObject)
		return validateAndGetOwner(owner)
	case *corev1.ServiceAccount:
		owner := metav1.GetControllerOf(resourceObject)
		return validateAndGetOwner(owner)
	case *corev1.Secret:
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
