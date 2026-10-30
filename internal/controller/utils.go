package controllers

import (
	"context"
	"fmt"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// directReader uses uncached reads when configured, including during conflict retries.
func (r *RabbitmqClusterReconciler) directReader() client.Reader {
	if r.APIReader != nil {
		return r.APIReader
	}
	return r.Client
}

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
	if _, exists := annotations[annotation]; !exists {
		return nil
	}
	base, ok := obj.DeepCopyObject().(client.Object)
	if !ok {
		return fmt.Errorf("deep copy of %T does not implement client.Object", obj)
	}
	delete(annotations, annotation)
	accessor.SetAnnotations(annotations)
	return r.Patch(ctx, obj, client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{}))
}

func (r *RabbitmqClusterReconciler) updateAnnotation(ctx context.Context, obj client.Object, namespace, objName, key, value string) error {
	return retry.OnError(
		retry.DefaultRetry,
		errorIsConflictOrNotFound, // StatefulSet needs time to be found after it got created
		func() error {
			// Decode into an empty object: JSON decoding can retain map entries
			// that were removed from the server object.
			current, copyBack, err := newAnnotationObject(obj)
			if err != nil {
				return err
			}
			if err := r.directReader().Get(ctx, types.NamespacedName{Namespace: namespace, Name: objName}, current); err != nil {
				return err
			}
			accessor, err := meta.Accessor(current)
			if err != nil {
				return err
			}
			annotations := accessor.GetAnnotations()
			if annotations == nil {
				annotations = make(map[string]string)
			}
			if existing, exists := annotations[key]; exists && existing == value {
				copyBack()
				return nil
			}
			base, ok := current.DeepCopyObject().(client.Object)
			if !ok {
				return fmt.Errorf("deep copy of %T does not implement client.Object", current)
			}
			annotations[key] = value
			accessor.SetAnnotations(annotations)
			if err := r.Patch(ctx, current, client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{})); err != nil {
				return err
			}
			copyBack()
			return nil
		})
}

// newAnnotationObject supplies a fresh decode target and a typed copy back
// for the resource types whose annotations the controller updates.
func newAnnotationObject(obj client.Object) (client.Object, func(), error) {
	switch target := obj.(type) {
	case *rabbitmqv1beta1.RabbitmqCluster:
		if target != nil {
			current := &rabbitmqv1beta1.RabbitmqCluster{}
			return current, func() { *target = *current }, nil
		}
	case *appsv1.StatefulSet:
		if target != nil {
			current := &appsv1.StatefulSet{}
			return current, func() { *target = *current }, nil
		}
	case *corev1.ConfigMap:
		if target != nil {
			current := &corev1.ConfigMap{}
			return current, func() { *target = *current }, nil
		}
	}
	return nil, nil, fmt.Errorf("unsupported or nil annotation object %T", obj)
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
