package controllers

import (
	"context"
	"fmt"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/resource"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

const queueRebalanceAnnotation = "rabbitmq.com/queueRebalanceNeededAt"

func (r *RabbitmqClusterReconciler) markForQueueRebalance(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) error {
	if rmq.ObjectMeta.Annotations == nil {
		rmq.ObjectMeta.Annotations = make(map[string]string)
	}

	if len(rmq.ObjectMeta.Annotations[queueRebalanceAnnotation]) > 0 {
		return nil
	}

	rmq.ObjectMeta.Annotations[queueRebalanceAnnotation] = time.Now().Format(time.RFC3339)
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
	pluginsConfig, err := r.configMap(ctx, rmq, rmq.ChildResourceName(resource.PluginsConfigName))
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

	// If the cluster has been marked as needing it, run rabbitmq-queues rebalance all
	if rmq.ObjectMeta.Annotations != nil && len(rmq.ObjectMeta.Annotations[queueRebalanceAnnotation]) > 0 {
		err = r.runQueueRebalanceCommand(ctx, rmq)
	}
	return 0, err
}

func (r *RabbitmqClusterReconciler) runQueueRebalanceCommand(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) error {
	podName := fmt.Sprintf("%s-0", rmq.ChildResourceName("server"))
	stdout, stderr, err := r.exec(rmq.Namespace, podName, "rabbitmq", "sh", "-c", "rabbitmq-queues rebalance all")
	if err != nil {
		r.Log.Error(err, "failed to run queue rebalance",
			"namespace", rmq.Namespace,
			"name", rmq.Name,
			"pod", podName,
			"command", "rabbitmq-queues rebalance all",
			"stdout", stdout,
			"stderr", stderr)
		return err
	}
	delete(rmq.ObjectMeta.Annotations, queueRebalanceAnnotation)
	return r.Update(ctx, rmq)
}

func statefulSetNeedsQueueRebalance(sts *appsv1.StatefulSet, rmq *rabbitmqv1beta1.RabbitmqCluster) bool {
	return statefulSetBeingUpdated(sts) &&
		!rmq.Spec.SkipPostDeploySteps &&
		*rmq.Spec.Replicas > 1
}

func allReplicasReadyAndUpdated(sts *appsv1.StatefulSet) bool {
	return sts.Status.ReadyReplicas == *sts.Spec.Replicas && !statefulSetBeingUpdated(sts)
}

func statefulSetBeingUpdated(sts *appsv1.StatefulSet) bool {
	return sts.Status.CurrentRevision != sts.Status.UpdateRevision
}
