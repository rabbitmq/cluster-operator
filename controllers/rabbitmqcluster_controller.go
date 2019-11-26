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
	"time"

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

	corev1 "k8s.io/api/core/v1"
)

var (
	ownerKey  = ".metadata.controller"
	ownerKind = "RabbitmqCluster"
	apiGVStr  = rabbitmqv1beta1.GroupVersion.String()
)

// RabbitmqClusterReconciler reconciles a RabbitmqCluster object
type RabbitmqClusterReconciler struct {
	client.Client
	Log                        logr.Logger
	Scheme                     *runtime.Scheme
	ServiceType                string
	ServiceAnnotations         map[string]string
	Image                      string
	ImagePullSecret            string
	PersistentStorageClassName string
	PersistentStorage          string
	Namespace                  string
	ResourceRequirements       resource.ResourceRequirements
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
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get
// +kubebuilder:rbac:groups=rabbitmq.pivotal.io,resources=rabbitmqclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.pivotal.io,resources=rabbitmqclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

func (r *RabbitmqClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := r.Log

	rabbitmqCluster, err := r.getRabbitmqCluster(req.NamespacedName)

	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		logger.Error(err, "Failed getting Rabbitmq cluster object")
		return reconcile.Result{}, err
	}

	instanceSpec, err := json.Marshal(rabbitmqCluster.Spec)
	if err != nil {
		logger.Error(err, "Failed to marshal cluster spec")
	}

	logger.Info(fmt.Sprintf("Start reconciling RabbitmqCluster \"%s\" in namespace \"%s\" with Spec: %+v",
		rabbitmqCluster.Name,
		rabbitmqCluster.Namespace,
		string(instanceSpec)))

	if rabbitmqCluster.Status.ClusterStatus == "" {
		r.updateStatus(rabbitmqCluster, "created")
	}

	// TODO refactor operatorRegistrySecret generation based on configured valued (stick it in config!)
	// https://www.pivotaltracker.com/story/show/169947561
	var operatorRegistrySecret *corev1.Secret
	if r.ImagePullSecret != "" && rabbitmqCluster.Spec.ImagePullSecret == "" {
		var err error
		operatorRegistrySecret, err = r.getImagePullSecret(types.NamespacedName{Namespace: r.Namespace, Name: r.ImagePullSecret})
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to find operator image pull secret: %v", err)
		}
	}

	defaultConfiguration := resource.DefaultConfiguration{
		ServiceAnnotations:         r.ServiceAnnotations,
		ServiceType:                r.ServiceType,
		Scheme:                     r.Scheme,
		OperatorRegistrySecret:     operatorRegistrySecret,
		ImageReference:             r.Image,
		ImagePullSecret:            r.ImagePullSecret,
		PersistentStorage:          r.PersistentStorage,
		PersistentStorageClassName: r.PersistentStorageClassName,
		ResourceRequirements:       r.ResourceRequirements,
	}

	resourceBuilder := resource.RabbitmqResourceBuilder{
		Instance:             rabbitmqCluster,
		DefaultConfiguration: defaultConfiguration,
	}
	resources, err := resourceBuilder.Resources()
	if err != nil {
		return reconcile.Result{}, err
	}

	if _, err = r.reconcileIngressService(resourceBuilder); err != nil {
		return reconcile.Result{}, err
	}

	if _, err = r.reconcileStatefulset(resourceBuilder); err != nil {
		return reconcile.Result{}, err
	}

	for _, re := range resources {
		if err := controllerutil.SetControllerReference(rabbitmqCluster, re.(metav1.Object), r.Scheme); err != nil {
			logger.Error(err, "Failed setting controller reference")
			return reconcile.Result{}, err
		}

		operationResult, err := controllerutil.CreateOrUpdate(context.TODO(), r, re, func() error { return nil })
		logger.Info(fmt.Sprintf("Operation Result \"%s\" for resource \"%s\" of Type %T",
			operationResult,
			re.(metav1.Object).GetName(),
			re.(metav1.Object)))

		if err != nil {
			logger.Error(err, "Failed to CreateOrUpdate")
			return reconcile.Result{}, err
		}
	}

	logger.Info(fmt.Sprintf("Finished reconciling cluster with name \"%s\" in namespace \"%s\"", rabbitmqCluster.Name, rabbitmqCluster.Namespace))

	if rabbitmqCluster.Status.ClusterStatus == "created" || rabbitmqCluster.Status.ClusterStatus == "running" {
		ready := r.ready(rabbitmqCluster)
		if ready {
			r.updateStatus(rabbitmqCluster, "running")
			return reconcile.Result{}, nil
		}
		r.updateStatus(rabbitmqCluster, "created")
	}

	return ctrl.Result{RequeueAfter: time.Second * 10}, nil
}

func (r *RabbitmqClusterReconciler) reconcileIngressService(builder resource.RabbitmqResourceBuilder) (reconcile.Result, error) {
	ingressService, err := builder.IngressService()
	if err != nil {
		return reconcile.Result{}, err
	}

	operationResult, err := controllerutil.CreateOrUpdate(context.TODO(), r, ingressService, func() error {

		builder.UpdateServiceParams(ingressService)
		return nil
	})

	if err != nil {
		return reconcile.Result{}, err
	}

	r.Log.Info(fmt.Sprintf("Operation Result \"%s\" for resource \"%s\" of Type Service",
		operationResult,
		ingressService.GetName()))

	return reconcile.Result{}, nil
}

func (r *RabbitmqClusterReconciler) reconcileStatefulset(builder resource.RabbitmqResourceBuilder) (reconcile.Result, error) {
	sts, err := builder.StatefulSet()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to generate StatefulSet: %v ", err)
	}

	operationResult, err := controllerutil.CreateOrUpdate(context.TODO(), r, sts, func() error {
		return builder.UpdateStatefulSetParams(sts)
	})

	if err != nil {
		return reconcile.Result{}, err
	}

	r.Log.Info(fmt.Sprintf("Operation Result \"%s\" for resource \"%s\" of Type StatefulSet",
		operationResult,
		sts.GetName()))

	return reconcile.Result{}, nil
}

func (r *RabbitmqClusterReconciler) updateStatus(rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster, status string) {
	rabbitmqCluster.Status.ClusterStatus = status
	err := r.Status().Update(context.TODO(), rabbitmqCluster)
	if err != nil {
		r.Log.Error(err, "Failed updating status")
	}
	r.Log.Info(fmt.Sprintf("RabbitmqCluster: %s is %s", rabbitmqCluster.Name, status))
}

func (r *RabbitmqClusterReconciler) ready(rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) bool {
	name := types.NamespacedName{
		Namespace: rabbitmqCluster.Namespace,
		Name:      rabbitmqCluster.ChildResourceName("ingress"),
	}
	if rabbitmqCluster.Spec.Service.Type == "LoadBalancer" {
		return r.loadBalancerReady(name) && r.endpointsReady(name, rabbitmqCluster.Spec.Replicas)
	}

	return r.endpointsReady(name, rabbitmqCluster.Spec.Replicas)
}

func (r *RabbitmqClusterReconciler) endpointsReady(name types.NamespacedName, replicas int) bool {
	endpoints := &corev1.Endpoints{}

	err := r.Get(context.TODO(), name, endpoints)
	if err != nil {
		r.Log.Error(err, "Failed to check if RabbitmqCluster endpoints are ready")
		return false
	}

	for _, e := range endpoints.Subsets {
		if len(e.NotReadyAddresses) == 0 && len(e.Addresses) == replicas {
			return true
		}
	}
	return false
}

func (r *RabbitmqClusterReconciler) loadBalancerReady(name types.NamespacedName) bool {
	svc := &corev1.Service{}

	err := r.Get(context.TODO(), name, svc)
	if err != nil {
		r.Log.Error(err, "Failed to check if RabbitmqCluster LoadBalancer service object is ready")
		return false
	}

	if len(svc.Status.LoadBalancer.Ingress) == 0 || svc.Status.LoadBalancer.Ingress[0].IP == "" {
		return false
	}

	return true
}

func (r *RabbitmqClusterReconciler) getRabbitmqCluster(NamespacedName types.NamespacedName) (*rabbitmqv1beta1.RabbitmqCluster, error) {
	rabbitmqClusterInstance := &rabbitmqv1beta1.RabbitmqCluster{}
	err := r.Get(context.TODO(), NamespacedName, rabbitmqClusterInstance)
	return rabbitmqClusterInstance, err
}

func (r *RabbitmqClusterReconciler) getImagePullSecret(NamespacedName types.NamespacedName) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := r.Get(context.TODO(), NamespacedName, secret)
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
