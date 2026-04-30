/*
RabbitMQ Cluster Operator

Copyright 2020 VMware, Inc. All Rights Reserved.

This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package rabbitmqclient

import (
	"context"
	"fmt"

	rabbithole "github.com/michaelklishin/rabbit-hole/v3"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RabbitmqClient represents a subset of the rabbithole.Client that the operator uses.
type RabbitmqClient interface {
	Overview() (*rabbithole.Overview, error)
	HealthCheckNodeIsQuorumCritical() (rabbithole.HealthCheckStatus, error)
	ListDeprecatedFeaturesUsed() ([]rabbithole.DeprecatedFeature, error)
}

// RabbitmqClientFactory creates a RabbitmqClient for a specific pod.
type RabbitmqClientFactory interface {
	GetClientForPod(ctx context.Context, k8sClient client.Reader, rmq *rabbitmqv1beta1.RabbitmqCluster, podName string) (RabbitmqClient, error)
}

// DefaultRabbitmqClientFactory is the default implementation of RabbitmqClientFactory.
type DefaultRabbitmqClientFactory struct{}

// GetClientForPod creates a real rabbithole client for a specific pod.
func (f *DefaultRabbitmqClientFactory) GetClientForPod(ctx context.Context, k8sClient client.Reader, rmq *rabbitmqv1beta1.RabbitmqCluster, podName string) (RabbitmqClient, error) {
	info, err := getClientInfoForPod(ctx, k8sClient, rmq, podName)
	if err != nil {
		return nil, err
	}

	var rabbitmqClient *rabbithole.Client
	if rmq.Spec.TLS.DisableNonTLSListeners {
		rabbitmqClient, err = rabbithole.NewTLSClient(info.BaseURL, info.Username, info.Password, info.Transport)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS rabbithole client for pod: %w", err)
		}
	} else {
		rabbitmqClient, err = rabbithole.NewClient(info.BaseURL, info.Username, info.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to create rabbithole client for pod: %w", err)
		}
	}

	return rabbitmqClient, nil
}
