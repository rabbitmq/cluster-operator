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
	ownerKind         = "Cluster"
	deletionFinalizer = "deletion.finalizers.clusters.rabbitmq.com"
)

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
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
// +kubebuilder:rbac:groups=rabbitmq.com,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=clusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

func (r *ClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	logger := r.Log

	fetchedCluster, err := r.getCluster(ctx, req.NamespacedName)

	if client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	} else if errors.IsNotFound(err) {
		// No need to requeue if the resource no longer exists
		return ctrl.Result{}, nil
	}

	cluster := rabbitmqv1beta1.MergeDefaults(*fetchedCluster)

	if !reflect.DeepEqual(fetchedCluster.Spec, cluster.Spec) {
		if err := r.Client.Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Resource has been marked for deletion
	if !cluster.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting Cluster",
			"namespace", cluster.Namespace,
			"name", cluster.Name)
		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, r.prepareForDeletion(ctx, cluster)
	}

	// TLS: check if specified, and if secret exists
	if cluster.TLSEnabled() {
		secretName := cluster.Spec.TLS.SecretName
		logger.Info("TLS set, looking for secret", "secret", secretName, "namespace", cluster.Namespace)

		// check if secret exists
		secret := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: cluster.Namespace, Name: secretName}, secret); err != nil {
			r.Recorder.Event(cluster, corev1.EventTypeWarning, "TLSError",
				fmt.Sprintf("Failed to get TLS secret in namespace %v: %v", cluster.Namespace, err.Error()))
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
			r.Recorder.Event(cluster, corev1.EventTypeWarning, "TLSError",
				fmt.Sprintf("The TLS secret %v in namespace %v must have the fields tls.crt and tls.key", secretName, cluster.Namespace))
			return ctrl.Result{}, errors.NewBadRequest("The TLS secret must have the fields tls.crt and tls.key")
		}
	}

	if err := r.addFinalizerIfNeeded(ctx, cluster); err != nil {
		return ctrl.Result{}, err
	}

	childResources, err := r.getChildResources(ctx, *cluster)

	if err != nil {
		return ctrl.Result{}, err
	}

	oldConditions := make([]status.ClusterCondition, len(cluster.Status.Conditions))
	copy(oldConditions, cluster.Status.Conditions)
	cluster.Status.SetConditions(childResources)

	if !reflect.DeepEqual(cluster.Status.Conditions, oldConditions) {
		err = r.Status().Update(ctx, cluster)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	instanceSpec, err := json.Marshal(cluster.Spec)
	if err != nil {
		logger.Error(err, "Failed to marshal cluster spec")
	}

	logger.Info("Start reconciling Cluster",
		"namespace", cluster.Namespace,
		"name", cluster.Name,
		"spec", string(instanceSpec))

	resourceBuilder := resource.RabbitmqResourceBuilder{
		Instance: cluster,
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
		if err := controllerutil.SetControllerReference(cluster, resource.(metav1.Object), r.Scheme); err != nil {
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
			r.logAndRecordOperationResult(cluster, resource, operationResult, err)
			return ctrl.Result{}, err
		}

		r.logAndRecordOperationResult(cluster, resource, operationResult, err)
		r.restartStatefulSetIfNeeded(ctx, resource, operationResult, cluster)
	}

	if err := r.setAdminStatus(ctx, cluster); err != nil {
		return ctrl.Result{}, err
	}

	if err, ok := r.allReplicasReady(ctx, cluster); !ok {
		// only enable plugins when all pods of the StatefulSet become ready
		// requeue request after 10 seconds without error
		logger.Info("Not all replicas ready yet; requeuing request to enable plugins on Cluster",
			"namespace", cluster.Namespace,
			"name", cluster.Name)
		return ctrl.Result{RequeueAfter: time.Second * 10}, err
	}

	if err := r.enablePlugins(cluster); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Finished reconciling Cluster",
		"namespace", cluster.Namespace,
		"name", cluster.Name)

	return ctrl.Result{}, nil
}

func (r *ClusterReconciler) setAdminStatus(ctx context.Context, rmq *rabbitmqv1beta1.Cluster) error {

	adminStatus := &rabbitmqv1beta1.ClusterAdmin{}

	serviceRef := &rabbitmqv1beta1.ClusterServiceReference{
		Name:      rmq.ChildResourceName("ingress"),
		Namespace: rmq.Namespace,
	}
	adminStatus.ServiceReference = serviceRef

	secretRef := &rabbitmqv1beta1.ClusterSecretReference{
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
func (r *ClusterReconciler) restartStatefulSetIfNeeded(ctx context.Context, resource runtime.Object, operationResult controllerutil.OperationResult, rmq *rabbitmqv1beta1.Cluster) {
	if _, ok := resource.(*corev1.ConfigMap); ok && operationResult == controllerutil.OperationResultUpdated {
		if err := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: rmq.ChildResourceName("server"), Namespace: rmq.Namespace}}
			if err := r.Get(ctx, types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace}, sts); err != nil {
				return err
			}
			if sts.Spec.Template.ObjectMeta.Annotations == nil {
				sts.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
			}
			sts.Spec.Template.ObjectMeta.Annotations["rabbitmq.com/restartAt"] = time.Now().Format(time.RFC3339)
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
func (r *ClusterReconciler) allReplicasReady(ctx context.Context, rmq *rabbitmqv1beta1.Cluster) (error, bool) {
	sts := &appsv1.StatefulSet{}

	if err := r.Get(ctx, types.NamespacedName{Name: rmq.ChildResourceName("server"), Namespace: rmq.Namespace}, sts); err != nil {
		return client.IgnoreNotFound(err), false
	}

	if sts.Status.ReadyReplicas < *sts.Spec.Replicas {
		return nil, false
	}

	return nil, true
}

// enablePlugins - helper function to set the list of enabled plugins in a given Cluster pods
// `rabbitmq-plugins set` disables plugins that are not in the provided list
func (r *ClusterReconciler) enablePlugins(rmq *rabbitmqv1beta1.Cluster) error {
	for i := int32(0); i < *rmq.Spec.Replicas; i++ {
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

	r.Log.Info("Successfully enabled plugins on Cluster",
		"namespace", rmq.Namespace,
		"name", rmq.Name)
	return nil
}

func (r *ClusterReconciler) exec(namespace, podName, containerName string, command ...string) (string, error) {
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
func (r *ClusterReconciler) logAndRecordOperationResult(rmq runtime.Object, resource runtime.Object, operationResult controllerutil.OperationResult, err error) {
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

func (r *ClusterReconciler) prepareForDeletion(ctx context.Context, cluster *rabbitmqv1beta1.Cluster) error {
	if containsString(cluster.ObjectMeta.Finalizers, deletionFinalizer) {
		if err := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			sts := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cluster.ChildResourceName("server"),
					Namespace: cluster.Namespace,
				},
			}
			// Add label on all Pods to be picked up in pre-stop hook via Downward API
			if err := r.addRabbitmqDeletionLabel(ctx, cluster); err != nil {
				return fmt.Errorf(fmt.Sprintf("Failed to add deletion markers to Cluster Pods: %s", err.Error()))
			}
			// Delete StatefulSet immediately after changing pod labels to minimize risk of them respawning. There is a window where the StatefulSet could respawn Pods without the deletion label in this order. But we can't delete it before because the DownwardAPI doesn't update once a Pod enters Terminating
			if err := r.Client.Delete(ctx, sts); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf(fmt.Sprintf("Cannot delete StatefulSet: %s", err.Error()))
			}

			return nil
		}); err != nil {
			r.Log.Error(err, "Cluster deletion")
		}

		if err := r.removeFinalizer(ctx, cluster); err != nil {
			r.Log.Error(err, "Failed to remove finalizer for deletion")
			return err
		}
	}
	return nil
}

func (r *ClusterReconciler) removeFinalizer(ctx context.Context, cluster *rabbitmqv1beta1.Cluster) error {
	if err := controllerutil.RemoveFinalizerWithError(cluster, deletionFinalizer); err != nil {
		return err
	}

	if err := r.Client.Update(ctx, cluster); err != nil {
		return err
	}

	return nil
}

func (r *ClusterReconciler) addRabbitmqDeletionLabel(ctx context.Context, cluster *rabbitmqv1beta1.Cluster) error {
	pods := &corev1.PodList{}
	selector, err := labels.Parse(fmt.Sprintf("app.kubernetes.io/name=%s", cluster.Name))
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

func (r *ClusterReconciler) addFinalizerIfNeeded(ctx context.Context, cluster *rabbitmqv1beta1.Cluster) error {
	// The Cluster is not marked for deletion (no deletion timestamp) but does not have the deletion finalizer
	if cluster.ObjectMeta.DeletionTimestamp.IsZero() && !containsString(cluster.ObjectMeta.Finalizers, deletionFinalizer) {
		if err := controllerutil.AddFinalizerWithError(cluster, deletionFinalizer); err != nil {
			return err
		}

		if err := r.Client.Update(ctx, cluster); err != nil {
			return err
		}
	}

	return nil
}

func (r *ClusterReconciler) getChildResources(ctx context.Context, rmq rabbitmqv1beta1.Cluster) ([]runtime.Object, error) {
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

func (r *ClusterReconciler) getCluster(ctx context.Context, NamespacedName types.NamespacedName) (*rabbitmqv1beta1.Cluster, error) {
	cluster := &rabbitmqv1beta1.Cluster{}
	err := r.Get(ctx, NamespacedName, cluster)
	return cluster, err
}

func (r *ClusterReconciler) getImagePullSecret(ctx context.Context, NamespacedName types.NamespacedName) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := r.Get(ctx, NamespacedName, secret)
	return secret, err
}

func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	for _, resource := range []runtime.Object{&appsv1.StatefulSet{}, &corev1.ConfigMap{}, &corev1.Service{}} {
		if err := mgr.GetFieldIndexer().IndexField(resource, ownerKey, addResourceToIndex); err != nil {
			return err
		}
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&rabbitmqv1beta1.Cluster{}).
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
