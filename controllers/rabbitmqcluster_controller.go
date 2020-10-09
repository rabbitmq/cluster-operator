/*
RabbitMQ Cluster Operator

Copyright 2020 VMware, Inc. All Rights Reserved.

This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
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

	"github.com/rabbitmq/cluster-operator/internal/resource"
	"github.com/rabbitmq/cluster-operator/internal/status"
	"k8s.io/apimachinery/pkg/api/errors"
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
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	apiGVStr = rabbitmqv1beta1.GroupVersion.String()
)

const (
	ownerKey                = ".metadata.controller"
	ownerKind               = "RabbitmqCluster"
	deletionFinalizer       = "deletion.finalizers.rabbitmqclusters.rabbitmq.com"
	pluginsUpdateAnnotation = "rabbitmq.com/pluginsUpdatedAt"
	postUpgradeAnnotation   = "rabbitmq.com/postUpgradeNeededAt"
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
	PodExecutor   KubectlExecutor
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
		if result, err := r.checkTLSSecrets(ctx, rabbitmqCluster); err != nil {
			return result, err
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
		if err = r.Status().Update(ctx, rabbitmqCluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	sts, err := r.statefulSet(ctx, rabbitmqCluster)
	// The StatefulSet may not have been created by this point, so ignore Not Found errors
	if client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}
	if statefulSetNeedsPostUpgrade(sts, rabbitmqCluster) {
		if err := r.markForPostUpgrade(ctx, rabbitmqCluster); err != nil {
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

		r.annotatePluginsConfigMapIfUpdated(ctx, builder, operationResult, rabbitmqCluster)
		if restarted := r.restartStatefulSetIfNeeded(ctx, builder, operationResult, rabbitmqCluster); restarted {
			return ctrl.Result{RequeueAfter: time.Second * 10}, nil
		}
	}

	// Set ReconcileSuccess to true here because all CRUD operations to Kube API related
	// to child resources returned no error
	rabbitmqCluster.Status.SetCondition(status.ReconcileSuccess, corev1.ConditionTrue, "Success", "Created or Updated all child resources")
	if writerErr := r.Status().Update(ctx, rabbitmqCluster); writerErr != nil {
		r.Log.Error(writerErr, "Error trying to Update Custom Resource status",
			"namespace", rabbitmqCluster.Namespace,
			"name", rabbitmqCluster.Name)
	}

	if err := r.setAdminStatus(ctx, rabbitmqCluster); err != nil {
		return ctrl.Result{}, err
	}

	// By this point the StatefulSet may have finished deploying. Run any
	// post-deploy steps if so, or requeue until the deployment is finished.
	requeueAfter, err := r.runPostDeployStepsIfNeeded(ctx, rabbitmqCluster)
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

func (r *RabbitmqClusterReconciler) checkTLSSecrets(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) (ctrl.Result, error) {
	logger := r.Log
	secretName := rabbitmqCluster.Spec.TLS.SecretName
	logger.Info("TLS set, looking for secret", "secret", secretName, "namespace", rabbitmqCluster.Namespace)

	// check if secret exists
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: rabbitmqCluster.Namespace, Name: secretName}, secret); err != nil {
		r.Recorder.Event(rabbitmqCluster, corev1.EventTypeWarning, "TLSError",
			fmt.Sprintf("Failed to get TLS secret %v in namespace %v: %v", secretName, rabbitmqCluster.Namespace, err.Error()))
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

	// Mutual TLS: check if CA certificate is stored in a separate secret
	if rabbitmqCluster.MutualTLSEnabled() {
		if !rabbitmqCluster.SingleTLSSecret() {
			secretName := rabbitmqCluster.Spec.TLS.CaSecretName
			logger.Info("mutual TLS set, looking for CA certificate secret", "secret", secretName, "namespace", rabbitmqCluster.Namespace)

			// check if secret exists
			secret = &corev1.Secret{}
			if err := r.Get(ctx, types.NamespacedName{Namespace: rabbitmqCluster.Namespace, Name: secretName}, secret); err != nil {
				r.Recorder.Event(rabbitmqCluster, corev1.EventTypeWarning, "TLSError",
					fmt.Sprintf("Failed to get CA certificate secret %v in namespace %v: %v", secretName, rabbitmqCluster.Namespace, err.Error()))
				return ctrl.Result{}, err
			}
		}
		// Mutual TLS: verify that CA certificate is present in secret
		_, hasCaCert := secret.Data[rabbitmqCluster.Spec.TLS.CaCertName]
		if !hasCaCert {
			r.Recorder.Event(rabbitmqCluster, corev1.EventTypeWarning, "TLSError",
				fmt.Sprintf("The TLS secret %v in namespace %v must have the field %v", rabbitmqCluster.Spec.TLS.CaSecretName, rabbitmqCluster.Namespace, rabbitmqCluster.Spec.TLS.CaCertName))

			return ctrl.Result{}, errors.NewBadRequest(fmt.Sprintf("The TLS secret must have the field %s", rabbitmqCluster.Spec.TLS.CaCertName))
		}
	}
	return ctrl.Result{}, nil
}

func (r *RabbitmqClusterReconciler) setAdminStatus(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) error {

	adminStatus := &rabbitmqv1beta1.RabbitmqClusterAdmin{}

	serviceRef := &rabbitmqv1beta1.RabbitmqClusterServiceReference{
		Name:      rmq.ChildResourceName("client"),
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

func (r *RabbitmqClusterReconciler) markForPostUpgrade(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) error {
	if rmq.ObjectMeta.Annotations == nil {
		rmq.ObjectMeta.Annotations = make(map[string]string)
	}

	if len(rmq.ObjectMeta.Annotations[postUpgradeAnnotation]) > 0 {
		return nil
	}

	rmq.ObjectMeta.Annotations[postUpgradeAnnotation] = time.Now().Format(time.RFC3339)
	if err := r.Update(ctx, rmq); err != nil {
		return err
	}
	return nil
}

func (r *RabbitmqClusterReconciler) runPostDeployStepsIfNeeded(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) (requeueAfter time.Duration, err error) {
	sts, err := r.statefulSet(ctx, rmq)
	if err != nil {
		return 0, err
	}
	if !allReplicasReadyAndUpdated(sts) {
		r.Log.Info("not all replicas ready yet; requeuing request to run post deploy steps",
			"namespace", rmq.Namespace,
			"name", rmq.Name)
		return 15 * time.Second, nil
	}

	// Retrieve the plugins config map, if it exists.
	pluginsConfig, err := r.pluginsConfigMap(ctx, rmq)
	if client.IgnoreNotFound(err) != nil {
		return 0, err
	}
	updatedRecently, err := pluginsConfigUpdatedRecently(pluginsConfig)
	if err != nil {
		return 0, err
	}
	if updatedRecently {
		// plugins configMap was updated very recently
		// give StatefulSet controller some time to trigger restart of StatefulSet if necessary
		// otherwise, there would be race conditions where we exec into containers losing the connection due to pods being terminated
		r.Log.Info("requeuing request to set plugins on RabbitmqCluster",
			"namespace", rmq.Namespace,
			"name", rmq.Name)
		return 2 * time.Second, nil
	}

	if pluginsConfig != nil {
		if err = r.runSetPluginsCommand(ctx, rmq, pluginsConfig); err != nil {
			return 0, err
		}
	}

	// If the cluster has been marked as needing it, run rabbitmq-upgrade post_upgrade
	if rmq.ObjectMeta.Annotations != nil && len(rmq.ObjectMeta.Annotations[postUpgradeAnnotation]) > 0 {
		err = r.runPostUpgradeCommand(ctx, rmq)
	}
	return 0, err
}

func (r *RabbitmqClusterReconciler) runPostUpgradeCommand(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) error {
	podName := fmt.Sprintf("%s-0", rmq.ChildResourceName("server"))
	stdout, stderr, err := r.exec(rmq.Namespace, podName, "rabbitmq", "sh", "-c", "rabbitmq-upgrade post_upgrade")
	if err != nil {
		r.Log.Error(err, "failed to run post-upgrade",
			"namespace", rmq.Namespace,
			"name", rmq.Name,
			"pod", podName,
			"command", "rabbitmq-upgrade post_upgrade",
			"stdout", stdout,
			"stderr", stderr)
		return err
	}
	delete(rmq.ObjectMeta.Annotations, postUpgradeAnnotation)
	return r.Update(ctx, rmq)
}

// There are 2 paths how plugins are set:
// 1. When StatefulSet is (re)started, the up-to-date plugins list (ConfigMap copied by the init container) is read by RabbitMQ nodes during node start up.
// 2. When the plugins ConfigMap is changed, 'rabbitmq-plugins set' updates the plugins on every node (without the need to re-start the nodes).
// This method implements the 2nd path.
func (r *RabbitmqClusterReconciler) runSetPluginsCommand(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster, configMap *corev1.ConfigMap) error {
	plugins := resource.NewRabbitmqPlugins(rmq.Spec.Rabbitmq.AdditionalPlugins)
	for i := int32(0); i < *rmq.Spec.Replicas; i++ {
		podName := fmt.Sprintf("%s-%d", rmq.ChildResourceName("server"), i)
		rabbitCommand := fmt.Sprintf("rabbitmq-plugins set %s", plugins.AsString(" "))
		stdout, stderr, err := r.exec(rmq.Namespace, podName, "rabbitmq", "sh", "-c", rabbitCommand)
		if err != nil {
			r.Log.Error(err, "failed to set plugins",
				"namespace", rmq.Namespace,
				"name", rmq.Name,
				"pod", podName,
				"command", rabbitCommand,
				"stdout", stdout,
				"stderr", stderr)
			return err
		}
	}
	r.Log.Info("successfully set plugins on RabbitmqCluster",
		"namespace", rmq.Namespace,
		"name", rmq.Name)

	delete(configMap.Annotations, pluginsUpdateAnnotation)
	if err := r.Update(ctx, configMap); err != nil {
		return err
	}

	return nil
}

// Adds an arbitrary annotation (rabbitmq.com/lastRestartAt) to the StatefulSet PodTemplate to trigger a StatefulSet restart
// if builder requires StatefulSet to be updated.
func (r *RabbitmqClusterReconciler) restartStatefulSetIfNeeded(
	ctx context.Context,
	builder resource.ResourceBuilder,
	operationResult controllerutil.OperationResult,
	rmq *rabbitmqv1beta1.RabbitmqCluster) (restarted bool) {

	if !(builder.UpdateRequiresStsRestart() && operationResult == controllerutil.OperationResultUpdated) {
		return false
	}

	if err := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: rmq.ChildResourceName("server"), Namespace: rmq.Namespace}}
		if err := r.Get(ctx, types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace}, sts); err != nil {
			return err
		}
		if sts.Spec.Template.ObjectMeta.Annotations == nil {
			sts.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
		}
		sts.Spec.Template.ObjectMeta.Annotations["rabbitmq.com/lastRestartAt"] = time.Now().Format(time.RFC3339)
		return r.Update(ctx, sts)
	}); err != nil {
		msg := fmt.Sprintf("failed to restart StatefulSet %s of Namespace %s; rabbitmq.conf configuration may be outdated", rmq.ChildResourceName("server"), rmq.Namespace)
		r.Log.Error(err, msg)
		r.Recorder.Event(rmq, corev1.EventTypeWarning, "FailedUpdate", msg)
		return false
	}

	msg := fmt.Sprintf("restarted StatefulSet %s of Namespace %s", rmq.ChildResourceName("server"), rmq.Namespace)
	r.Log.Info(msg)
	r.Recorder.Event(rmq, corev1.EventTypeNormal, "SuccessfulUpdate", msg)
	return true
}

func (r *RabbitmqClusterReconciler) statefulSet(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) (*v1.StatefulSet, error) {
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: rmq.ChildResourceName("server"), Namespace: rmq.Namespace}, sts); err != nil {
		return nil, err
	}
	return sts, nil
}

func (r *RabbitmqClusterReconciler) pluginsConfigMap(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: rmq.Namespace, Name: rmq.ChildResourceName(resource.PluginsConfig)}, configMap); err != nil {
		return nil, err
	}
	return configMap, nil
}

// Annotates the plugins ConfigMap if it was updated such that 'rabbitmq-plugins set' will be called on the RabbitMQ nodes at a later point in time
func (r *RabbitmqClusterReconciler) annotatePluginsConfigMapIfUpdated(
	ctx context.Context,
	builder resource.ResourceBuilder,
	operationResult controllerutil.OperationResult,
	rmq *rabbitmqv1beta1.RabbitmqCluster) {

	if _, ok := builder.(*resource.RabbitmqPluginsConfigMapBuilder); !ok {
		return
	}
	if operationResult != controllerutil.OperationResultUpdated {
		return
	}

	if retryOnConflictErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		configMap := corev1.ConfigMap{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: rmq.Namespace, Name: rmq.ChildResourceName(resource.PluginsConfig)}, &configMap); err != nil {
			return client.IgnoreNotFound(err)
		}
		if configMap.Annotations == nil {
			configMap.Annotations = make(map[string]string)
		}
		configMap.Annotations[pluginsUpdateAnnotation] = time.Now().Format(time.RFC3339)
		return r.Update(ctx, &configMap)
	}); retryOnConflictErr != nil {
		msg := fmt.Sprintf("Failed to annotate ConfigMap %s of Namespace %s; enabled_plugins may be outdated", rmq.ChildResourceName(resource.PluginsConfig), rmq.Namespace)
		r.Log.Error(retryOnConflictErr, msg)
		r.Recorder.Event(rmq, corev1.EventTypeWarning, "FailedUpdate", msg)
	}
}

func (r *RabbitmqClusterReconciler) exec(namespace, podName, containerName string, command ...string) (string, string, error) {
	return r.PodExecutor.Exec(r.Clientset, r.ClusterConfig, namespace, podName, containerName, command...)
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

func (r *RabbitmqClusterReconciler) prepareForDeletion(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) error {
	if controllerutil.ContainsFinalizer(rabbitmqCluster, deletionFinalizer) {
		if err := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			sts := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      rabbitmqCluster.ChildResourceName("server"),
					Namespace: rabbitmqCluster.Namespace,
				},
			}
			// Add label on all Pods to be picked up in pre-stop hook via Downward API
			if err := r.addRabbitmqDeletionLabel(ctx, rabbitmqCluster); err != nil {
				return fmt.Errorf("failed to add deletion markers to RabbitmqCluster Pods: %s", err.Error())
			}
			// Delete StatefulSet immediately after changing pod labels to minimize risk of them respawning.
			// There is a window where the StatefulSet could respawn Pods without the deletion label in this order.
			// But we can't delete it before because the DownwardAPI doesn't update once a Pod enters Terminating.
			if err := r.Client.Delete(ctx, sts); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("cannot delete StatefulSet: %s", err.Error())
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
	controllerutil.RemoveFinalizer(rabbitmqCluster, deletionFinalizer)
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
			return fmt.Errorf("cannot Update Pod %s in Namespace %s: %s", pod.Name, pod.Namespace, err.Error())
		}
	}

	return nil
}

func (r *RabbitmqClusterReconciler) addFinalizerIfNeeded(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) error {
	// The RabbitmqCluster is not marked for deletion (no deletion timestamp) but does not have the deletion finalizer
	if rabbitmqCluster.ObjectMeta.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(rabbitmqCluster, deletionFinalizer) {
		controllerutil.AddFinalizer(rabbitmqCluster, deletionFinalizer)
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

func allReplicasReadyAndUpdated(sts *v1.StatefulSet) bool {
	return sts.Status.ReadyReplicas == *sts.Spec.Replicas && !statefulSetBeingUpdated(sts)
}

func statefulSetBeingUpdated(sts *v1.StatefulSet) bool {
	return sts.Status.CurrentRevision != sts.Status.UpdateRevision
}

func statefulSetNeedsPostUpgrade(sts *v1.StatefulSet, rmq *rabbitmqv1beta1.RabbitmqCluster) bool {
	return sts != nil &&
		statefulSetBeingUpdated(sts) &&
		!rmq.Spec.SkipPostUpgradeSteps &&
		*rmq.Spec.Replicas > 1
}

func pluginsConfigUpdatedRecently(cfg *corev1.ConfigMap) (bool, error) {
	if cfg == nil {
		return false, nil
	}
	pluginsUpdatedAt, ok := cfg.Annotations[pluginsUpdateAnnotation]
	if !ok {
		return false, nil // plugins configMap was not updated
	}

	annotationTime, err := time.Parse(time.RFC3339, pluginsUpdatedAt)
	if err != nil {
		return false, err
	}
	return time.Since(annotationTime).Seconds() < 2, nil
}

type KubectlExecutor interface {
	Exec(clientset *kubernetes.Clientset, clusterConfig *rest.Config, namespace, podName, containerName string, command ...string) (string, string, error)
}

func NewPodExecutor() KubectlExecutor { return &podExecutor{} }

type podExecutor struct{}

func (p *podExecutor) Exec(clientset *kubernetes.Clientset, clusterConfig *rest.Config, namespace, podName, containerName string, command ...string) (string, string, error) {
	request := clientset.CoreV1().RESTClient().
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

	exec, err := remotecommand.NewSPDYExecutor(clusterConfig, "POST", request.URL())
	if err != nil {
		return "", "", err
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
		return stdOut.String(), stdErr.String(), err
	}
	if stdErr.Len() > 0 {
		return stdOut.String(), stdErr.String(), fmt.Errorf("%v", stdErr)
	}

	return stdOut.String(), "", nil
}
