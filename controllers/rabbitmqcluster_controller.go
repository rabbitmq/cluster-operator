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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	Log                logr.Logger
	Scheme             *runtime.Scheme
	ServiceType        string
	ServiceAnnotations map[string]string
	ImageRepository    string
	ImagePullSecret    string
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
// +kubebuilder:rbac:groups=rabbitmq.pivotal.io,resources=rabbitmqclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.pivotal.io,resources=rabbitmqclusters/status,verbs=get;update;patch

func (r *RabbitmqClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	var pvcBaseName string
	_ = context.Background()
	logger := r.Log

	instance := &rabbitmqv1beta1.RabbitmqCluster{}
	err := r.Get(context.TODO(), req.NamespacedName, instance)

	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		logger.Error(err, "failed getting Rabbitmq cluster object")
		return reconcile.Result{}, err
	}
	rabbitmqSecret, err := resource.GenerateSecret(*instance)

	if err != nil {
		logger.Error(err, "failed to generate secret")
		return reconcile.Result{}, err
	}

	instanceSpec, err := json.Marshal(instance.Spec)
	if err != nil {
		logger.Error(err, "failed to marshal cluster spec")
	}

	logger.Info(fmt.Sprintf("Start reconciling RabbitmqCluster \"%s\" in namespace \"%s\" with Spec: %+v",
		instance.Name,
		instance.Namespace,
		string(instanceSpec)))

	service := resource.GenerateService(*instance, r.ServiceType, r.ServiceAnnotations)
	resources := []runtime.Object{
		resource.GenerateStatefulSet(*instance, r.ImageRepository, r.ImagePullSecret),
		resource.GenerateConfigMap(*instance),
		service,
		rabbitmqSecret,
	}

	serviceSpec, err := json.Marshal(service.Spec)
	if err != nil {
		logger.Error(err, "failed to marshal service spec")
	}
	logger.V(1).Info(fmt.Sprintf("Rabbitmq service \"%s\" has Spec: %v", service.ObjectMeta.Name, string(serviceSpec)))

	for _, re := range resources {
		switch sts := re.(type) {
		case *appsv1.StatefulSet:
			pvcBaseName = sts.Spec.VolumeClaimTemplates[0].ObjectMeta.Name
		}

		if err := controllerutil.SetControllerReference(instance, re.(metav1.Object), r.Scheme); err != nil {
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

	for i := 0; i < instance.Spec.Replicas; i++ {
		pvcObjectKey := metav1.ObjectMeta{
			Namespace: instance.Namespace,
			Name:      fmt.Sprintf("%s-p-%s-%d", pvcBaseName, instance.Name, i),
		}
		patchedPvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: pvcObjectKey,
		}
		if err := controllerutil.SetControllerReference(instance, patchedPvc, r.Scheme); err != nil {
			logger.Error(err, "Failed setting controller reference")
			return reconcile.Result{}, err
		}

		originalPvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: pvcObjectKey,
		}

		mergeFromPatch := client.MergeFrom(originalPvc)
		patch, err := mergeFromPatch.Data(patchedPvc)
		if err != nil {
			logger.Error(err, "failed to generate Patch object")
			return reconcile.Result{}, err
		}

		err = r.Patch(context.TODO(), originalPvc, client.ConstantPatch(types.StrategicMergePatchType, patch))
		if err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "Failed to patch PVC")
			return reconcile.Result{}, err
		} else if apierrors.IsNotFound(err) {
			logger.Info(fmt.Sprintf("PVC \"%s\" not found", pvcObjectKey.Name))
		} else {
			logger.Info(fmt.Sprintf("Successfully patched PVC \"%s\"", pvcObjectKey.Name))
		}
	}

	logger.Info(fmt.Sprintf("Finished reconciling cluster with name \"%s\" in namespace \"%s\"", instance.Name, instance.Namespace))

	return reconcile.Result{}, err
}

func (r *RabbitmqClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {

	if err := mgr.GetFieldIndexer().IndexField(&appsv1.StatefulSet{}, ownerKey, func(rawObj runtime.Object) []string {
		statefulSet := rawObj.(*appsv1.StatefulSet)
		owner := metav1.GetControllerOf(statefulSet)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != apiGVStr || owner.Kind != ownerKind {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&rabbitmqv1beta1.RabbitmqCluster{}).
		Owns(&appsv1.StatefulSet{}).
		Complete(r)
}
