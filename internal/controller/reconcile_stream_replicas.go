package controllers

import (
	"context"
	"fmt"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// reconcileStreamReplicas grows stream queue membership onto newly added
// broker nodes after a scale-up. Unlike quorum queues, RabbitMQ has no
// broker-side reconciler for stream replicas (no equivalent of
// quorum_queue.continuous_membership_reconciliation), so the operator has to
// explicitly run `rabbitmq-streams add_replica` for each stream and each
// node that is not yet a member. This is best-effort: failures are logged
// and recorded as events, but do not fail the overall reconcile loop.
func (r *RabbitmqClusterReconciler) reconcileStreamReplicas(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster, desiredSts *appsv1.StatefulSet) bool {
	logger := ctrl.LoggerFrom(ctx)

	rabbitClient, err := r.RabbitmqClientFactory.GetClientForService(ctx, r.APIReader, rmq)
	if err != nil {
		logger.V(1).Info("failed to get management client to reconcile stream replicas", "error", err)
		return true
	}

	queues, err := rabbitClient.ListQueues()
	if err != nil {
		logger.V(1).Info("failed to list queues to reconcile stream replicas", "error", err)
		return true
	}

	desiredReplicas := int32(1)
	if desiredSts.Spec.Replicas != nil {
		desiredReplicas = *desiredSts.Spec.Replicas
	}
	desiredNodes := make([]string, 0, desiredReplicas)
	for i := int32(0); i < desiredReplicas; i++ {
		desiredNodes = append(desiredNodes, streamNodeName(rmq, i))
	}

	coordinatorPod := fmt.Sprintf("%s-0", rmq.ChildResourceName("server"))
	retry := false
	for _, queue := range queues {
		if queue.Type != "stream" {
			continue
		}
		members := make(map[string]bool, len(queue.Members))
		for _, member := range queue.Members {
			members[member] = true
		}
		for _, node := range desiredNodes {
			if members[node] {
				continue
			}
			cmd := fmt.Sprintf("rabbitmq-streams add_replica --vhost %q %q %q", queue.Vhost, queue.Name, node)
			stdout, stderr, err := r.exec(rmq.Namespace, coordinatorPod, "rabbitmq", "sh", "-c", cmd)
			if err != nil {
				msg := fmt.Sprintf("failed to add stream replica for %q on %q", queue.Name, node)
				logger.Error(err, msg, "stdout", stdout, "stderr", stderr)
				r.Recorder.Event(rmq, corev1.EventTypeWarning, "FailedStreamReplicaAdd", fmt.Sprintf("%s: %s", msg, err.Error()))
				retry = true
				continue
			}
			logger.Info("added stream replica", "queue", queue.Name, "vhost", queue.Vhost, "node", node)
		}
	}
	return retry
}

// streamNodeName builds the Erlang node name of broker pod index i, in the
// same form RabbitMQ reports queue members (rabbit@<pod>.<nodes-svc>.<namespace>).
func streamNodeName(rmq *rabbitmqv1beta1.RabbitmqCluster, index int32) string {
	podName := fmt.Sprintf("%s-%d", rmq.ChildResourceName("server"), index)
	return fmt.Sprintf("rabbit@%s.%s.%s", podName, rmq.ChildResourceName("nodes"), rmq.Namespace)
}
