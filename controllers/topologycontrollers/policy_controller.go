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

// +kubebuilder:rbac:groups=rabbitmq.com,resources=policies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=policies/finalizers,verbs=update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=policies/status,verbs=get;update;patch

type PolicyReconciler struct{}

// creates or updates a given policy using rabbithole client.PutPolicy
func (r *PolicyReconciler) DeclareFunc(ctx context.Context, client rabbitmqclient.Client, obj rabbitmqv1beta1.TopologyResource) error {
	policy := obj.(*rabbitmqv1beta1.Policy)
	generatePolicy, err := internal.GeneratePolicy(policy)
	if err != nil {
		return fmt.Errorf("failed to generate Policy: %w", err)
	}
	return validateResponse(client.PutPolicy(policy.Spec.Vhost, policy.Spec.Name, *generatePolicy))
}

// deletes policy from rabbitmq server
// if server responds with '404' Not Found, it logs and does not requeue on error
func (r *PolicyReconciler) DeleteFunc(ctx context.Context, client rabbitmqclient.Client, obj rabbitmqv1beta1.TopologyResource) error {
	logger := ctrl.LoggerFrom(ctx)
	policy := obj.(*rabbitmqv1beta1.Policy)
	err := validateResponseForDeletion(client.DeletePolicy(policy.Spec.Vhost, policy.Spec.Name))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find policy in rabbitmq server; already deleted", "policy", policy.Spec.Name)
	} else if err != nil {
		return err
	}
	return nil
}
