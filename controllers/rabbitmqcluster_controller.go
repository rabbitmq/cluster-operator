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

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
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
	Log                         logr.Logger
	Scheme                      *runtime.Scheme
	ServiceType                 string
	ServiceAnnotations          map[string]string
	ImageRepository             string
	ImagePullSecret             string
	PersistenceStorageClassName string
	PersistenceStorage          string
}

// the rbac rule requires an empty row at the end to render
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get
// +kubebuilder:rbac:groups=rabbitmq.pivotal.io,resources=rabbitmqclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.pivotal.io,resources=rabbitmqclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

func (r *RabbitmqClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log

	rabbitmqClusterInstance, err := r.getRabbitmqClusterInstance(req.NamespacedName)

	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		logger.Error(err, "failed getting Rabbitmq cluster object")
		return reconcile.Result{}, err
	}

	instanceSpec, err := json.Marshal(rabbitmqClusterInstance.Spec)
	if err != nil {
		logger.Error(err, "failed to marshal cluster spec")
	}

	logger.Info(fmt.Sprintf("Start reconciling RabbitmqCluster \"%s\" in namespace \"%s\" with Spec: %+v",
		rabbitmqClusterInstance.Name,
		rabbitmqClusterInstance.Namespace,
		string(instanceSpec)))

	resources, err := r.getResources(rabbitmqClusterInstance)
	if err != nil {
		logger.Error(err, "failed to generate resources")
		return reconcile.Result{}, err
	}

	for _, re := range resources {
		if err := controllerutil.SetControllerReference(rabbitmqClusterInstance, re.(metav1.Object), r.Scheme); err != nil {
			logger.Error(err, "Failed setting controller reference")
			return reconcile.Result{}, err
		}

		operationResult, err := controllerutil.CreateOrUpdate(context.TODO(), r, re, func() error { return nil })
		logger.Info(fmt.Sprintf("Operation Result \"%s\" for resource \"%s\" of Type %T",
			operationResult,
			re.(metav1.Object).GetName(),
			re.(metav1.Object)))

		if err != nil {
			logger.Error(err, "failed to CreateOrUpdate")
			return reconcile.Result{}, err
		}
	}

	logger.Info(fmt.Sprintf("Finished reconciling cluster with name \"%s\" in namespace \"%s\"", rabbitmqClusterInstance.Name, rabbitmqClusterInstance.Namespace))

	return reconcile.Result{}, err
}

func (r *RabbitmqClusterReconciler) getResources(rabbitmqClusterInstance *rabbitmqv1beta1.RabbitmqCluster) ([]runtime.Object, error) {
	rabbitmqSecret, err := resource.GenerateSecret(*rabbitmqClusterInstance)
	if err != nil {
		return nil, fmt.Errorf("failed to generate secret: %v ", err)
	}

	statefulSet, err := resource.GenerateStatefulSet(*rabbitmqClusterInstance, r.ImageRepository, r.ImagePullSecret, r.PersistenceStorageClassName, r.PersistenceStorage, r.Scheme)
	if err != nil {
		return nil, fmt.Errorf("failed to generate StatefulSet: %v ", err)
	}

	resources := []runtime.Object{
		statefulSet,
		resource.GeneratePluginsConfigMap(*rabbitmqClusterInstance),
		resource.GenerateRabbitmqConfigMap(*rabbitmqClusterInstance),
		resource.GenerateIngressService(*rabbitmqClusterInstance, r.ServiceType, r.ServiceAnnotations),
		resource.GenerateHeadlessService(*rabbitmqClusterInstance),
		rabbitmqSecret,
		resource.GenerateServiceAccount(*rabbitmqClusterInstance),
		resource.GenerateRole(*rabbitmqClusterInstance),
		resource.GenerateRoleBinding(*rabbitmqClusterInstance),
	}

	return resources, nil
}

func (r *RabbitmqClusterReconciler) getRabbitmqClusterInstance(NamespacedName types.NamespacedName) (*rabbitmqv1beta1.RabbitmqCluster, error) {
	rabbitmqClusterInstance := &rabbitmqv1beta1.RabbitmqCluster{}
	err := r.Get(context.TODO(), NamespacedName, rabbitmqClusterInstance)
	return rabbitmqClusterInstance, err
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
	default:
		return nil
	}
}
