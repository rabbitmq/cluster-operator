/*
Copyright 2019 Pivotal.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/client-go/tools/record"
	"reflect"

	"k8s.io/apimachinery/pkg/types"
	clientretry "k8s.io/client-go/util/retry"

	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/status"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	ownerKey  = ".metadata.controller"
	ownerKind = "RabbitmqCluster"
	apiGVStr  = rabbitmqv1beta1.GroupVersion.String()
)

// RabbitmqClusterReconciler reconciles a RabbitmqCluster object
type RabbitmqClusterReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Namespace string
	Recorder  record.EventRecorder
}

// the rbac rule requires an empty row at the end to render
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=endpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rabbitmq.pivotal.io,resources=rabbitmqclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.pivotal.io,resources=rabbitmqclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

func (r *RabbitmqClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	logger := r.Log

	fetchedRabbitmqCluster, err := r.getRabbitmqCluster(ctx, req.NamespacedName)

	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		logger.Error(err, "Failed getting Rabbitmq cluster object")
		return reconcile.Result{}, err
	}

	rabbitmqCluster := rabbitmqv1beta1.MergeDefaults(*fetchedRabbitmqCluster)

	if !reflect.DeepEqual(fetchedRabbitmqCluster.Spec, rabbitmqCluster.Spec) {
		err := r.Client.Update(ctx, rabbitmqCluster)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	childResources, err := r.getChildResources(ctx, *rabbitmqCluster)

	if err != nil {
		logger.Error(err, "Error getting child resources")
		return reconcile.Result{}, err
	}

	oldConditions := make([]status.RabbitmqClusterCondition, len(rabbitmqCluster.Status.Conditions))
	copy(oldConditions, rabbitmqCluster.Status.Conditions)
	rabbitmqCluster.Status.SetConditions(childResources)

	if !reflect.DeepEqual(rabbitmqCluster.Status.Conditions, oldConditions) {
		err = r.Status().Update(ctx, rabbitmqCluster)
		if err != nil {
			logger.Error(err, "Failed to update the RabbitmqCluster status")
			return ctrl.Result{}, err
		}
	}

	instanceSpec, err := json.Marshal(rabbitmqCluster.Spec)
	if err != nil {
		logger.Error(err, "Failed to marshal cluster spec")
	}

	logger.Info(fmt.Sprintf("Start reconciling RabbitmqCluster \"%s\" in namespace \"%s\" with Spec: %+v",
		rabbitmqCluster.Name,
		rabbitmqCluster.Namespace,
		string(instanceSpec)))

	resourceBuilder := resource.RabbitmqResourceBuilder{
		Instance: rabbitmqCluster,
		Scheme:   r.Scheme,
	}

	builders, err := resourceBuilder.ResourceBuilders()
	if err != nil {
		return reconcile.Result{}, err
	}

	for _, builder := range builders {
		resource, err := builder.Build()
		if err != nil {
			return reconcile.Result{}, err
		}

		//TODO this should be done in the builders
		if err := controllerutil.SetControllerReference(rabbitmqCluster, resource.(metav1.Object), r.Scheme); err != nil {
			logger.Error(err, "Failed setting controller reference")
			return reconcile.Result{}, err
		}

		var operationResult controllerutil.OperationResult
		if err := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			var apiError error
			operationResult, apiError = controllerutil.CreateOrUpdate(ctx, r, resource, func() error {
				return builder.Update(resource)
			})

			return apiError
		}); err != nil {
			r.logAndRecordOperationResult(rabbitmqCluster, resource, operationResult, err)
			return reconcile.Result{}, err
		}

		r.logAndRecordOperationResult(rabbitmqCluster, resource, operationResult, err)
	}

	logger.Info(fmt.Sprintf("Finished reconciling cluster with name \"%s\" in namespace \"%s\"", rabbitmqCluster.Name, rabbitmqCluster.Namespace))

	return ctrl.Result{}, nil
}

func (r *RabbitmqClusterReconciler) logAndRecordOperationResult(rmq runtime.Object, resource runtime.Object, operationResult controllerutil.OperationResult, err error) {
	if operationResult == controllerutil.OperationResultCreated {
		if err != nil {
			msg := fmt.Sprintf("failed to create resource %s of Type %T", resource.(metav1.Object).GetName(), resource.(metav1.Object))

			r.Log.Error(err, msg)
			r.Recorder.Event(rmq, corev1.EventTypeWarning, "FailedCreate", msg)
		} else {
			msg := fmt.Sprintf("created resource %s of Type %T", resource.(metav1.Object).GetName(), resource.(metav1.Object))

			r.Log.Info(msg)
			r.Recorder.Event(rmq, corev1.EventTypeNormal, "SuccessfulCreate", msg)
		}
	}

	if operationResult == controllerutil.OperationResultUpdated {
		if err != nil {
			msg := fmt.Sprintf("failed to update resource %s of Type %T", resource.(metav1.Object).GetName(), resource.(metav1.Object))

			r.Log.Error(err, msg)
			r.Recorder.Event(rmq, corev1.EventTypeWarning, "FailedUpdate", msg)
		} else {
			msg := fmt.Sprintf("updated resource %s of Type %T", resource.(metav1.Object).GetName(), resource.(metav1.Object))

			r.Log.Info(msg)
			r.Recorder.Event(rmq, corev1.EventTypeNormal, "SuccessfulUpdate", msg)
		}
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
		types.NamespacedName{Name: rmq.ChildResourceName("ingress"), Namespace: rmq.Namespace},
		endPoints); err != nil && !errors.IsNotFound(err) {
		return nil, err
	} else if errors.IsNotFound(err) {
		endPoints = nil
	}

	return []runtime.Object{sts, endPoints}, nil
}

func (r *RabbitmqClusterReconciler) getRabbitmqCluster(ctx context.Context, NamespacedName types.NamespacedName) (*rabbitmqv1beta1.RabbitmqCluster, error) {
	rabbitmqClusterInstance := &rabbitmqv1beta1.RabbitmqCluster{}
	err := r.Get(ctx, NamespacedName, rabbitmqClusterInstance)
	return rabbitmqClusterInstance, err
}

func (r *RabbitmqClusterReconciler) getImagePullSecret(ctx context.Context, NamespacedName types.NamespacedName) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := r.Get(ctx, NamespacedName, secret)
	return secret, err
}

func (r *RabbitmqClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	for _, resource := range []runtime.Object{&appsv1.StatefulSet{}, &corev1.ConfigMap{}, &corev1.Service{}} {
		if err := mgr.GetFieldIndexer().IndexField(resource, ownerKey, addResourceToIndex); err != nil {
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
		if owner == nil {
			return nil
		}
		if owner.APIVersion != apiGVStr || owner.Kind != ownerKind {
			return nil
		}
		return []string{owner.Name}
	case *corev1.ConfigMap:
		owner := metav1.GetControllerOf(resourceObject)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != apiGVStr || owner.Kind != ownerKind {
			return nil
		}
		return []string{owner.Name}
	case *corev1.Service:
		owner := metav1.GetControllerOf(resourceObject)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != apiGVStr || owner.Kind != ownerKind {
			return nil
		}
		return []string{owner.Name}
	case *rbacv1.Role:
		owner := metav1.GetControllerOf(resourceObject)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != apiGVStr || owner.Kind != ownerKind {
			return nil
		}
		return []string{owner.Name}
	case *rbacv1.RoleBinding:
		owner := metav1.GetControllerOf(resourceObject)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != apiGVStr || owner.Kind != ownerKind {
			return nil
		}
		return []string{owner.Name}
	case *corev1.ServiceAccount:
		owner := metav1.GetControllerOf(resourceObject)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != apiGVStr || owner.Kind != ownerKind {
			return nil
		}
		return []string{owner.Name}
	case *corev1.Secret:
		owner := metav1.GetControllerOf(resourceObject)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != apiGVStr || owner.Kind != ownerKind {
			return nil
		}
		return []string{owner.Name}

	default:
		return nil
	}
}
