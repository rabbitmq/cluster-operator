package controllers

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

func (r *RabbitmqClusterReconciler) exec(namespace, podName, containerName string, command ...string) (string, string, error) {
	return r.PodExecutor.Exec(r.Clientset, r.ClusterConfig, namespace, podName, containerName, command...)
}

func (r *RabbitmqClusterReconciler) deleteAnnotation(ctx context.Context, obj client.Object, annotation string) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	annotations := accessor.GetAnnotations()
	if annotations == nil {
		return nil
	}
	delete(annotations, annotation)
	accessor.SetAnnotations(annotations)
	return r.Update(ctx, obj)
}

func (r *RabbitmqClusterReconciler) updateAnnotation(ctx context.Context, obj client.Object, namespace, objName, key, value string) error {
	return retry.OnError(
		retry.DefaultRetry,
		errorIsConflictOrNotFound, // StatefulSet needs time to be found after it got created
		func() error {
			if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: objName}, obj); err != nil {
				return err
			}
			accessor, err := meta.Accessor(obj)
			if err != nil {
				return err
			}
			annotations := accessor.GetAnnotations()
			if annotations == nil {
				annotations = make(map[string]string)
			}
			annotations[key] = value
			accessor.SetAnnotations(annotations)
			return r.Update(ctx, obj)
		})
}

func errorIsConflictOrNotFound(err error) bool {
	return errors.IsConflict(err) || errors.IsNotFound(err)
}

func (r *RabbitmqClusterReconciler) statefulSet(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) (*appsv1.StatefulSet, error) {
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: rmq.ChildResourceName("server"), Namespace: rmq.Namespace}, sts); err != nil {
		return nil, err
	}
	return sts, nil
}

func (r *RabbitmqClusterReconciler) configMap(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster, name string) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: rmq.Namespace, Name: name}, configMap); err != nil {
		return nil, err
	}
	return configMap, nil
}
