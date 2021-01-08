package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/resource"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	pluginsUpdateAnnotation = "rabbitmq.com/pluginsUpdatedAt"
	serverConfAnnotation    = "rabbitmq.com/serverConfUpdatedAt"
	stsRestartAnnotation    = "rabbitmq.com/lastRestartAt"
	stsCreateAnnotation     = "rabbitmq.com/createdAt"
)

// Annotates an object depending on object type and operationResult.
// These annotations are temporary markers used in later reconcile loops to perform some action (such as restarting the StatefulSet or executing RabbitMQ CLI commands)
func (r *RabbitmqClusterReconciler) annotateIfNeeded(ctx context.Context, logger logr.Logger, builder resource.ResourceBuilder, operationResult controllerutil.OperationResult, rmq *rabbitmqv1beta1.RabbitmqCluster) error {
	var (
		obj           client.Object
		objName       string
		annotationKey string
	)

	switch builder.(type) {

	case *resource.RabbitmqPluginsConfigMapBuilder:
		if operationResult != controllerutil.OperationResultUpdated {
			return nil
		}
		obj = &corev1.ConfigMap{}
		objName = rmq.ChildResourceName(resource.PluginsConfigName)
		annotationKey = pluginsUpdateAnnotation

	case *resource.ServerConfigMapBuilder:
		if operationResult != controllerutil.OperationResultUpdated {
			return nil
		}
		obj = &corev1.ConfigMap{}
		objName = rmq.ChildResourceName(resource.ServerConfigMapName)
		annotationKey = serverConfAnnotation

	case *resource.StatefulSetBuilder:
		if operationResult != controllerutil.OperationResultCreated {
			return nil
		}
		obj = &appsv1.StatefulSet{}
		objName = rmq.ChildResourceName("server")
		annotationKey = stsCreateAnnotation

	default:
		return nil
	}

	if err := r.updateAnnotation(ctx, obj, rmq.Namespace, objName, annotationKey, time.Now().Format(time.RFC3339)); err != nil {
		msg := "failed to annotate " + objName
		logger.Error(err, msg)
		r.Recorder.Event(rmq, corev1.EventTypeWarning, "FailedUpdate", msg)
		return err
	}

	logger.Info("successfully annotated")
	return nil
}

// Adds an arbitrary annotation to the sts PodTemplate to trigger a sts restart.
// It compares annotation "rabbitmq.com/serverConfUpdatedAt" from server-conf configMap and annotation "rabbitmq.com/lastRestartAt" from sts
// to determine whether to restart sts.
func (r *RabbitmqClusterReconciler) restartStatefulSetIfNeeded(ctx context.Context, logger logr.Logger, rmq *rabbitmqv1beta1.RabbitmqCluster) (time.Duration, error) {
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
		msg := fmt.Sprintf("failed to restart StatefulSet %s; rabbitmq.conf configuration may be outdated", rmq.ChildResourceName("server"))
		logger.Error(err, msg)
		r.Recorder.Event(rmq, corev1.EventTypeWarning, "FailedUpdate", msg)
		// failed to restart sts; return error to requeue request
		return 0, err
	}

	msg := fmt.Sprintf("restarted StatefulSet %s", rmq.ChildResourceName("server"))
	logger.Info(msg)
	r.Recorder.Event(rmq, corev1.EventTypeNormal, "SuccessfulUpdate", msg)

	return 0, nil
}

func pluginsConfigUpdatedRecently(cfg *corev1.ConfigMap) (bool, error) {
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
