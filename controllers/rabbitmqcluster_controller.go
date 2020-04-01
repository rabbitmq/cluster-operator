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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"k8s.io/client-go/tools/record"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
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
	apiGVStr = rabbitmqv1beta1.GroupVersion.String()
)

const (
	ownerKey          = ".metadata.controller"
	ownerKind         = "RabbitmqCluster"
	deletionFinalizer = "deletion.finalizers.rabbitmq"
)

type PodExecute func(string, string, string, ...string) (string, error)

// RabbitmqClusterReconciler reconciles a RabbitmqCluster object
type RabbitmqClusterReconciler struct {
	Exec PodExecute
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Namespace string
	Recorder  record.EventRecorder
}

// the rbac rule requires an empty row at the end to render
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
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
		logger.Error(err, "Failed getting Rabbitmq cluster object")
		// No need to requeue if the resource no longer exists, otherwise we'll
		// requeue the error.
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	rabbitmqCluster := rabbitmqv1beta1.MergeDefaults(*fetchedRabbitmqCluster)

	if !reflect.DeepEqual(fetchedRabbitmqCluster.Spec, rabbitmqCluster.Spec) {
		if err := r.Client.Update(ctx, rabbitmqCluster); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	// Check if deletion timestamp is set
	if err := r.AddFinalizerIfNeeded(ctx, rabbitmqCluster); err != nil {
		return ctrl.Result{}, err
	}

	// Resource has been marked for deletion
	if !rabbitmqCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info(fmt.Sprintf("Deleting RabbitmqCluster \"%s\" in namespace \"%s\"",
			rabbitmqCluster.Name,
			rabbitmqCluster.Namespace))
		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, r.PrepareForDeletion(ctx, rabbitmqCluster)
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

// logAndRecordOperationResult - helper function to log and record events with message and error
// it logs and records 'updated' and 'created' OperationResult, and ignores OperationResult 'unchanged'
func (r *RabbitmqClusterReconciler) logAndRecordOperationResult(rmq runtime.Object, resource runtime.Object, operationResult controllerutil.OperationResult, err error) {
	if operationResult == controllerutil.OperationResultCreated && err == nil {
		msg := fmt.Sprintf("created resource %s of Type %T", resource.(metav1.Object).GetName(), resource.(metav1.Object))
		r.Log.Info(msg)
		r.Recorder.Event(rmq, corev1.EventTypeNormal, "SuccessfulCreate", msg)
	}

	if operationResult == controllerutil.OperationResultCreated && err != nil {
		msg := fmt.Sprintf("failed to create resource %s of Type %T", resource.(metav1.Object).GetName(), resource.(metav1.Object))
		r.Log.Error(err, msg)
		r.Recorder.Event(rmq, corev1.EventTypeWarning, "FailedCreate", msg)
	}

	if operationResult == controllerutil.OperationResultUpdated && err == nil {
		msg := fmt.Sprintf("updated resource %s of Type %T", resource.(metav1.Object).GetName(), resource.(metav1.Object))
		r.Log.Info(msg)
		r.Recorder.Event(rmq, corev1.EventTypeNormal, "SuccessfulUpdate", msg)
	}

	if operationResult == controllerutil.OperationResultUpdated && err != nil {
		msg := fmt.Sprintf("failed to update resource %s of Type %T", resource.(metav1.Object).GetName(), resource.(metav1.Object))
		r.Log.Error(err, msg)
		r.Recorder.Event(rmq, corev1.EventTypeWarning, "FailedUpdate", msg)
	}
}

// Helper functions to check and remove string from a slice of strings.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func (r *RabbitmqClusterReconciler) PrepareForDeletion(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) error {
	if containsString(rabbitmqCluster.ObjectMeta.Finalizers, deletionFinalizer) {
		if err := r.ShutdownRabbitmq(ctx, rabbitmqCluster); err != nil {
			r.Log.Error(err, "Failed to shutdown RabbitmqCluster")
			return err
		}

		if err := r.RemoveFinalizer(ctx, rabbitmqCluster); err != nil {
			r.Log.Error(err, "Failed to remove finalizer for deletion")
			return err
		}
	}
	return nil
}

func (r *RabbitmqClusterReconciler) RemoveFinalizer(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) error {
	if err := controllerutil.RemoveFinalizerWithError(rabbitmqCluster, deletionFinalizer); err != nil {
		return err
	}

	if err := r.Client.Update(ctx, rabbitmqCluster); err != nil {
		return err
	}

	return nil
}

func (r *RabbitmqClusterReconciler) ShutdownRabbitmq(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) error {
	sts := &appsv1.StatefulSet{}
	err := r.Client.Get(ctx,
		types.NamespacedName{Name: rabbitmqCluster.ChildResourceName("server"), Namespace: rabbitmqCluster.Namespace},
		sts)
	if client.IgnoreNotFound(err) != nil {
		return err
	}

	if err == nil {
		// Delete StatefulSet so Pods aren't restarted after shutdown
		if err := r.Client.Delete(ctx, sts); err != nil {
			return err
		}

		// Shutdown RabbitMQ on all nodes so Pre-Stop hook terminates immediately
		for i := 0; i < int(*sts.Spec.Replicas); i++ {
			//TODO: Can we be specific about when we want to retry this? Currently all output (even successful shutdown - exit code 69) returns to stdErr so we can't handle errors properly.
			if _, err := r.Exec(rabbitmqCluster.Namespace, fmt.Sprintf("%s-%d", sts.Name, i), sts.Spec.Template.Spec.Containers[0].Name, "sh", "-c", "rabbitmqctl shutdown"); err != nil {
				r.Log.Info(fmt.Sprintf("Error returned from rabbitmqctl shutdown: %s", err.Error()))
			}
		}
	}

	return nil
}

func (r *RabbitmqClusterReconciler) AddFinalizerIfNeeded(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) error {
	// The RabbitmqCluster is not marked for deletion (no deletion timestamp) but does not have the deletion finalizer
	if rabbitmqCluster.ObjectMeta.DeletionTimestamp.IsZero() && !containsString(rabbitmqCluster.ObjectMeta.Finalizers, deletionFinalizer) {
		if err := controllerutil.AddFinalizerWithError(rabbitmqCluster, deletionFinalizer); err != nil {
			return err
		}

		if err := r.Client.Update(ctx, rabbitmqCluster); err != nil {
			return err
		}
	}

	return nil
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

func Exec(namespace, podName, containerName string, command ...string) (string, error) {
	var kubeClient *kubernetes.Clientset
	var inClusterConfig *rest.Config
	var err error
	inClusterConfig, err = rest.InClusterConfig()
	if err != nil {
		return "", err
	}

	kubeClient = kubernetes.NewForConfigOrDie(inClusterConfig)

	request := kubeClient.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdout:    true,
			Stderr:    true,
			Stdin:     false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(inClusterConfig, "POST", request.URL())
	if err != nil {
		return "", err
	}

	stdOut := bytes.Buffer{}
	stdErr := bytes.Buffer{}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: bufio.NewWriter(&stdOut),
		Stderr: bufio.NewWriter(&stdErr),
		Stdin:  nil,
		Tty:    false,
	})

	if err != nil {
		return "", err
	}

	if stdErr.Len() > 0 {
		return "", fmt.Errorf("%v", stdErr)
	}

	return stdOut.String(), nil
}
