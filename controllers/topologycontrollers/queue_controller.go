/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package topologycontrollers

import (
	"context"
	"errors"
	"fmt"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"

	"github.com/rabbitmq/cluster-operator/internal/rabbitmqclient"
	"github.com/rabbitmq/cluster-operator/internal/topology"
	ctrl "sigs.k8s.io/controller-runtime"
)

// +kubebuilder:rbac:groups=rabbitmq.com,resources=queues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=queues/finalizers,verbs=update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=queues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters/status,verbs=get
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;create;patch

type QueueReconciler struct{}

func (r *QueueReconciler) DeclareFunc(ctx context.Context, client rabbitmqclient.Client, obj rabbitmqv1beta1.TopologyResource) error {
	queue := obj.(*rabbitmqv1beta1.Queue)
	queueSettings, err := internal.GenerateQueueSettings(queue)
	if err != nil {
		return fmt.Errorf("failed to generate queue settings: %w", err)
	}
	return validateResponse(client.DeclareQueue(queue.Spec.Vhost, queue.Spec.Name, *queueSettings))
}

// deletes queue from rabbitmq server
// if server responds with '404' Not Found, it logs and does not requeue on error
// queues could be deleted manually or gone because of AutoDelete
func (r *QueueReconciler) DeleteFunc(ctx context.Context, client rabbitmqclient.Client, obj rabbitmqv1beta1.TopologyResource) error {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Deleting queues from ReconcilerFunc DeleteObj")

	queue := obj.(*rabbitmqv1beta1.Queue)
	err := validateResponseForDeletion(client.DeleteQueue(queue.Spec.Vhost, queue.Spec.Name))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find queue in rabbitmq server; already deleted", "queue", queue.Spec.Name)
	} else if err != nil {
		return err
	}
	return nil
}
