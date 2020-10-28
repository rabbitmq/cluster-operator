package controllers

import (
	"context"
	"fmt"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/resource"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"
)

const (
	pluginsUpdateAnnotation = "rabbitmq.com/pluginsUpdatedAt"
	serverConfAnnotation    = "rabbitmq.com/serverConfUpdatedAt"
	stsRestartAnnotation    = "rabbitmq.com/lastRestartAt"
)

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

// Annotates the plugins ConfigMap or the server-conf ConfigMap
// annotations later used to indicate whether to call 'rabbitmq-plugins set' or to restart the sts
func (r *RabbitmqClusterReconciler) annotateConfigMapIfUpdated(ctx context.Context, builder resource.ResourceBuilder, operationResult controllerutil.OperationResult, rmq *rabbitmqv1beta1.RabbitmqCluster) error {
	if operationResult != controllerutil.OperationResultUpdated {
		return nil
	}

	var configMap, annotationKey string
	switch builder.(type) {
	case *resource.RabbitmqPluginsConfigMapBuilder:
		configMap = rmq.ChildResourceName(resource.PluginsConfigName)
		annotationKey = pluginsUpdateAnnotation
	case *resource.ServerConfigMapBuilder:
		configMap = rmq.ChildResourceName(resource.ServerConfigMapName)
		annotationKey = serverConfAnnotation
	default:
		return nil
	}

	if err := r.annotateConfigMap(ctx, rmq.Namespace, configMap, annotationKey, time.Now().Format(time.RFC3339)); err != nil {
		msg := fmt.Sprintf("Failed to annotate ConfigMap %s of Namespace %s; %s may be outdated", configMap, rmq.Namespace, rmq.Name)
		r.Log.Error(err, msg)
		r.Recorder.Event(rmq, corev1.EventTypeWarning, "FailedUpdate", msg)
		return err
	}

	r.Log.Info("successfully annotated", "ConfigMap", configMap, "Namespace", rmq.Namespace)
	return nil
}

// Adds an arbitrary annotation to the sts PodTemplate to trigger a sts restart
// it compares annotation "rabbitmq.com/serverConfUpdatedAt" from server-conf configMap and annotation "rabbitmq.com/lastRestartAt" from sts
// to determine whether to restart sts
func (r *RabbitmqClusterReconciler) restartStatefulSetIfNeeded(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) (time.Duration, error) {
	serverConf, err := r.configMap(ctx, rmq, rmq.ChildResourceName(resource.ServerConfigMapName))
	if err != nil {
		// requeue request after 10s if unable to find server-conf configmap, else return the error
		return 10 * time.Second, client.IgnoreNotFound(err)
	}

	serverConfigUpdatedAt, ok := serverConf.Annotations[serverConfAnnotation]
	if !ok {
		// server-conf configmap hasn't been updated; no need to restart sts
		return 0, nil
	}

	sts, err := r.statefulSet(ctx, rmq)
	if err != nil {
		// requeue request after 10s if unable to find sts, else return the error
		return 10 * time.Second, client.IgnoreNotFound(err)
	}

	stsRestartedAt, ok := sts.Spec.Template.ObjectMeta.Annotations[stsRestartAnnotation]
	if ok && stsRestartedAt > serverConfigUpdatedAt {
		// sts was updated after the last server-conf configmap update; no need to restart sts
		return 0, nil
	}

	if err := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: rmq.ChildResourceName("server"), Namespace: rmq.Namespace}}
		if err := r.Get(ctx, types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace}, sts); err != nil {
			return err
		}
		if sts.Spec.Template.ObjectMeta.Annotations == nil {
			sts.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
		}
		sts.Spec.Template.ObjectMeta.Annotations[stsRestartAnnotation] = time.Now().Format(time.RFC3339)
		return r.Update(ctx, sts)
	}); err != nil {
		msg := fmt.Sprintf("failed to restart StatefulSet %s of Namespace %s; rabbitmq.conf configuration may be outdated", rmq.ChildResourceName("server"), rmq.Namespace)
		r.Log.Error(err, msg)
		r.Recorder.Event(rmq, corev1.EventTypeWarning, "FailedUpdate", msg)
		// failed to restart sts; return error to requeue request
		return 0, err
	}

	msg := fmt.Sprintf("restarted StatefulSet %s of Namespace %s", rmq.ChildResourceName("server"), rmq.Namespace)
	r.Log.Info(msg)
	r.Recorder.Event(rmq, corev1.EventTypeNormal, "SuccessfulUpdate", msg)

	return 0, nil
}

func (r *RabbitmqClusterReconciler) annotateConfigMap(ctx context.Context, namespace, name, key, value string) error {
	if retryOnConflictErr := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		configMap := corev1.ConfigMap{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &configMap); err != nil {
			return client.IgnoreNotFound(err)
		}
		if configMap.Annotations == nil {
			configMap.Annotations = make(map[string]string)
		}
		configMap.Annotations[key] = value
		return r.Update(ctx, &configMap)
	}); retryOnConflictErr != nil {
		return retryOnConflictErr
	}
	return nil
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
