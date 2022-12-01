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

// +kubebuilder:rbac:groups=rabbitmq.com,resources=exchanges,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=exchanges/finalizers,verbs=update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=exchanges/status,verbs=get;update;patch

type ExchangeReconciler struct{}

func (r *ExchangeReconciler) DeclareFunc(ctx context.Context, client rabbitmqclient.Client, obj rabbitmqv1beta1.TopologyResource) error {
	exchange := obj.(*rabbitmqv1beta1.Exchange)
	settings, err := internal.GenerateExchangeSettings(exchange)
	if err != nil {
		return fmt.Errorf("failed to generate exchange settings: %w", err)
	}
	return validateResponse(client.DeclareExchange(exchange.Spec.Vhost, exchange.Spec.Name, *settings))
}

// deletes exchange from rabbitmq server
// if server responds with '404' Not Found, it logs and does not requeue on error
func (r *ExchangeReconciler) DeleteFunc(ctx context.Context, client rabbitmqclient.Client, obj rabbitmqv1beta1.TopologyResource) error {
	logger := ctrl.LoggerFrom(ctx)
	exchange := obj.(*rabbitmqv1beta1.Exchange)
	err := validateResponseForDeletion(client.DeleteExchange(exchange.Spec.Vhost, exchange.Spec.Name))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find exchange in rabbitmq server; already deleted", "exchange", exchange.Spec.Name)
	} else if err != nil {
		return err
	}
	return nil
}
