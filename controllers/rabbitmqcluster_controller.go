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
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/status"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"

	"k8s.io/apimachinery/pkg/types"

	clientretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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
	deletionFinalizer = "deletion.finalizers.rabbitmqclusters.rabbitmq.pivotal.io"
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
}

// the rbac rule requires an empty row at the end to render
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
// +kubebuilder:rbac:groups="",resources=pods,verbs=update;get;list;watch;
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

	if client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	} else if errors.IsNotFound(err) {
		// No need to requeue if the resource no longer exists
		return ctrl.Result{}, nil
	}

	rabbitmqCluster := rabbitmqv1beta1.MergeDefaults(*fetchedRabbitmqCluster)

	if !reflect.DeepEqual(fetchedRabbitmqCluster.Spec, rabbitmqCluster.Spec) {
		if err := r.Client.Update(ctx, rabbitmqCluster); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Resource has been marked for deletion
	if !rabbitmqCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting RabbitmqCluster",
			"namespace", rabbitmqCluster.Namespace,
			"name", rabbitmqCluster.Name)
		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, r.prepareForDeletion(ctx, rabbitmqCluster)
	}

	// TLS: check if specified, and if secret exists
	if rabbitmqCluster.TLSEnabled() {
		secretName := rabbitmqCluster.Spec.TLS.SecretName
		logger.Info("TLS set, looking for secret", "secret", secretName, "namespace", rabbitmqCluster.Namespace)

		// check if secret exists
		secret := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: rabbitmqCluster.Namespace, Name: secretName}, secret); err != nil {
			r.Recorder.Event(rabbitmqCluster, corev1.EventTypeWarning, "TLSError",
				fmt.Sprintf("Failed to get TLS secret in namespace %v: %v", rabbitmqCluster.Namespace, err.Error()))
			// retry after 10 seconds if not found
			if errors.IsNotFound(err) {
				return ctrl.Result{RequeueAfter: 10 * time.Second}, err
			}

			return ctrl.Result{}, err
		}

		// check if secret has the right keys
		_, hasTLSKey := secret.Data["tls.key"]
		_, hasTLSCert := secret.Data["tls.crt"]
		if !hasTLSCert || !hasTLSKey {
			r.Recorder.Event(rabbitmqCluster, corev1.EventTypeWarning, "TLSError",
				fmt.Sprintf("The TLS secret %v in namespace %v must have the fields tls.crt and tls.key", secretName, rabbitmqCluster.Namespace))
			return ctrl.Result{}, errors.NewBadRequest("The TLS secret must have the fields tls.crt and tls.key")
		}
	}

	if err := r.addFinalizerIfNeeded(ctx, rabbitmqCluster); err != nil {
		return ctrl.Result{}, err
	}

	childResources, err := r.getChildResources(ctx, *rabbitmqCluster)

	if err != nil {
		return ctrl.Result{}, err
	}

	oldConditions := make([]status.RabbitmqClusterCondition, len(rabbitmqCluster.Status.Conditions))
	copy(oldConditions, rabbitmqCluster.Status.Conditions)
	rabbitmqCluster.Status.SetConditions(childResources)

	if !reflect.DeepEqual(rabbitmqCluster.Status.Conditions, oldConditions) {
		err = r.Status().Update(ctx, rabbitmqCluster)
		if err != nil {
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

		//TODO this should be done in the builders
		if err := controllerutil.SetControllerReference(rabbitmqCluster, resource.(metav1.Object), r.Scheme); err != nil {
			return ctrl.Result{}, err
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
			return ctrl.Result{}, err
		}

		r.logAndRecordOperationResult(rabbitmqCluster, resource, operationResult, err)
		r.restartStatefulSetIfNeeded(ctx, resource, operationResult, rabbitmqCluster)
	}

	if err := r.setAdminStatus(ctx, rabbitmqCluster); err != nil {
		return ctrl.Result{}, err
	}

	if err, ok := r.allReplicasReady(ctx, rabbitmqCluster); !ok {
		// only enable plugins when all pods of the StatefulSet become ready
		// requeue request after 10 seconds without error
		logger.Info("Not all replicas are ready; unable to configure plugins on RabbitmqCluster",
			"namespace", rabbitmqCluster.Namespace,
			"name", rabbitmqCluster.Name)
		return ctrl.Result{RequeueAfter: time.Second * 10}, err
	}

	if err := r.enablePlugins(rabbitmqCluster); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Finished reconciling RabbitmqCluster",
		"namespace", rabbitmqCluster.Namespace,
		"name", rabbitmqCluster.Name)

	return ctrl.Result{}, nil
}

func (r *RabbitmqClusterReconciler) setAdminStatus(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) error {

	adminStatus := &rabbitmqv1beta1.RabbitmqClusterAdmin{}

	serviceRef := &rabbitmqv1beta1.RabbitmqClusterServiceReference{
		Name:      rmq.ChildResourceName("ingress"),
		Namespace: rmq.Namespace,
	}
	adminStatus.ServiceReference = serviceRef

	secretRef := &rabbitmqv1beta1.RabbitmqClusterSecretReference{
		Name:      rmq.ChildResourceName(resource.AdminSecretName),
		Namespace: rmq.Namespace,
		Keys: map[string]string{
			"username": "username",
			"password": "password",
		},
	}
	adminStatus.SecretReference = secretRef

	if !reflect.DeepEqual(rmq.Status.Admin, adminStatus) {
		rmq.Status.Admin = adminStatus
		if err := r.Status().Update(ctx, rmq); err != nil {
			return err
		}
	}

	return nil
}

// restartStatefulSetIfNeeded - helper function that annotate the StatefulSet PodTemplate with current timestamp
// to trigger a restart of the all pods in the StatefulSet when ConfigMap is updated
func (r *RabbitmqClusterReconciler) restartStatefulSetIfNeeded(ctx context.Context, resource runtime.Object, operationResult controllerutil.OperationResult, rmq *rabbitmqv1beta1.RabbitmqCluster) {
	if _, ok := resource.(*corev1.ConfigMap); ok && operationResult == controllerutil.OperationResultUpdated {
		if err := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: rmq.ChildResourceName("server"), Namespace: rmq.Namespace}}
			if err := r.Get(ctx, types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace}, sts); err != nil {
				return err
			}
			if sts.Spec.Template.ObjectMeta.Annotations == nil {
				sts.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
			}
			sts.Spec.Template.ObjectMeta.Annotations["rabbitmq.pivotal.io/restartAt"] = time.Now().Format(time.RFC3339)
			return r.Update(ctx, sts)
		}); err != nil {
			msg := fmt.Sprintf("Failed to restart StatefulSet %s of Namespace %s; rabbitmq.conf configuration may be outdated", rmq.ChildResourceName("server"), rmq.Namespace)
			r.Log.Error(err, msg)
			r.Recorder.Event(rmq, corev1.EventTypeWarning, "FailedUpdate", msg)
		}
		msg := fmt.Sprintf("Restarted StatefulSet %s of Namespace %s", rmq.ChildResourceName("server"), rmq.Namespace)
		r.Log.Info(msg)
		r.Recorder.Event(rmq, corev1.EventTypeNormal, "SuccessfulUpdate", msg)
	}
}

// allReplicasReady - helper function that checks if StatefulSet replicas are all ready
func (r *RabbitmqClusterReconciler) allReplicasReady(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) (error, bool) {
	sts := &appsv1.StatefulSet{}

	if err := r.Get(ctx, types.NamespacedName{Name: rmq.ChildResourceName("server"), Namespace: rmq.Namespace}, sts); err != nil {
		return client.IgnoreNotFound(err), false
	}

	if sts.Status.ReadyReplicas < *sts.Spec.Replicas {
		return nil, false
	}

	return nil, true
}

// enablePlugins - helper function to set the list of enabled plugins in a given RabbitmqCluster pods
// `rabbitmq-plugins set` disables plugins that are not in the provided list
func (r *RabbitmqClusterReconciler) enablePlugins(rmq *rabbitmqv1beta1.RabbitmqCluster) error {
	for i := 0; i < int(rmq.Spec.Replicas); i++ {
		podName := fmt.Sprintf("%s-%d", rmq.ChildResourceName("server"), i)
		rabbitCommand := fmt.Sprintf("rabbitmq-plugins set %s",
			strings.Join(resource.AppendIfUnique(resource.RequiredPlugins, rmq.Spec.Rabbitmq.AdditionalPlugins), " "))

		output, err := r.exec(rmq.Namespace, podName, "rabbitmq", "sh", "-c", rabbitCommand)

		if err != nil {
			r.Log.Error(err, fmt.Sprintf(
				"Failed to enable plugins on pod %s in namespace %s, running command %s with output %s",
				podName, rmq.Namespace, rabbitCommand, output))

			return err
		}
	}

	r.Log.Info("Successfully enabled plugins on RabbitmqCluster",
		"namespace", rmq.Namespace,
		"name", rmq.Name)
	return nil
}

func (r *RabbitmqClusterReconciler) exec(namespace, podName, containerName string, command ...string) (string, error) {
	request := r.Clientset.CoreV1().RESTClient().
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

	exec, err := remotecommand.NewSPDYExecutor(r.ClusterConfig, "POST", request.URL())
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

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func (r *RabbitmqClusterReconciler) prepareForDeletion(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) error {
	if containsString(rabbitmqCluster.ObjectMeta.Finalizers, deletionFinalizer) {
		if err := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			sts := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      rabbitmqCluster.ChildResourceName("server"),
					Namespace: rabbitmqCluster.Namespace,
				},
			}
			// Add label on all Pods to be picked up in pre-stop hook via Downward API
			if err := r.addRabbitmqDeletionLabel(ctx, rabbitmqCluster); err != nil {
				return fmt.Errorf(fmt.Sprintf("Failed to add deletion markers to RabbitmqCluster Pods: %s", err.Error()))
			}
			// Delete StatefulSet immediately after changing pod labels to minimize risk of them respawning. There is a window where the StatefulSet could respawn Pods without the deletion label in this order. But we can't delete it before because the DownwardAPI doesn't update once a Pod enters Terminating
			if err := r.Client.Delete(ctx, sts); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf(fmt.Sprintf("Cannot delete StatefulSet: %s", err.Error()))
			}

			return nil
		}); err != nil {
			r.Log.Error(err, "RabbitmqCluster deletion")
		}

		if err := r.removeFinalizer(ctx, rabbitmqCluster); err != nil {
			r.Log.Error(err, "Failed to remove finalizer for deletion")
			return err
		}
	}
	return nil
}

func (r *RabbitmqClusterReconciler) removeFinalizer(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) error {
	if err := controllerutil.RemoveFinalizerWithError(rabbitmqCluster, deletionFinalizer); err != nil {
		return err
	}

	if err := r.Client.Update(ctx, rabbitmqCluster); err != nil {
		return err
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
	}

	if err := r.Client.List(ctx, pods, &listOptions); err != nil {
		return err
	}

	for i := 0; i < len(pods.Items); i++ {
		pod := &pods.Items[i]
		pod.Labels[resource.DeletionMarker] = "true"
		if err := r.Client.Update(ctx, pod); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf(fmt.Sprintf("Cannot Update Pod %s in Namespace %s: %s", pod.Name, pod.Namespace, err.Error()))
		}
	}

	return nil
}

func (r *RabbitmqClusterReconciler) addFinalizerIfNeeded(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) error {
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
