/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package topologycontrollers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"reflect"

	"github.com/go-logr/logr"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	"github.com/rabbitmq/cluster-operator/internal/rabbitmqclient"
	"github.com/rabbitmq/cluster-operator/internal/topology"
	ctrl "sigs.k8s.io/controller-runtime"
)

// +kubebuilder:rbac:groups=rabbitmq.com,resources=bindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=bindings/finalizers,verbs=update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=bindings/status,verbs=get;update;patch

type BindingReconciler struct{}

func (r *BindingReconciler) DeclareFunc(ctx context.Context, client rabbitmqclient.Client, obj rabbitmqv1beta1.TopologyResource) error {
	binding := obj.(*rabbitmqv1beta1.Binding)
	info, err := internal.GenerateBindingInfo(binding)
	if err != nil {
		return fmt.Errorf("failed to generate binding info: %w", err)
	}
	return validateResponse(client.DeclareBinding(binding.Spec.Vhost, *info))
}

// deletes binding from rabbitmq server; bindings have no name; server needs BindingInfo to delete them
// when server responds with '404' Not Found, it logs and does not requeue on error
// if no binding argument is set, generating properties key by using internal.GeneratePropertiesKey
// if binding arguments are set, list all bindings between source/destination to find the binding; if it failed to find corresponding binding, it assumes that the binding is already deleted and returns no error
func (r *BindingReconciler) DeleteFunc(ctx context.Context, client rabbitmqclient.Client, obj rabbitmqv1beta1.TopologyResource) error {
	logger := ctrl.LoggerFrom(ctx)
	binding := obj.(*rabbitmqv1beta1.Binding)
	var info *rabbithole.BindingInfo
	var err error
	if binding.Spec.Arguments != nil {
		info, err = r.findBindingInfo(logger, binding, client)
		if err != nil {
			return err
		}
		if info == nil {
			logger.Info("cannot find the corresponding binding info in rabbitmq server; binding already deleted")
			return nil
		}
	} else {
		info, err = internal.GenerateBindingInfo(binding)
		if err != nil {
			return fmt.Errorf("failed to generate binding info: %w", err)
		}
		info.PropertiesKey = internal.GeneratePropertiesKey(binding)
	}

	err = validateResponseForDeletion(client.DeleteBinding(binding.Spec.Vhost, *info))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find binding in rabbitmq server; already deleted")
	} else if err != nil {
		return err
	}

	return nil
}

func (r *BindingReconciler) findBindingInfo(logger logr.Logger, binding *rabbitmqv1beta1.Binding, client rabbitmqclient.Client) (*rabbithole.BindingInfo, error) {
	logger.Info("binding arguments set; listing bindings from server to complete deletion")
	arguments := make(map[string]interface{})
	if binding.Spec.Arguments != nil {
		if err := json.Unmarshal(binding.Spec.Arguments.Raw, &arguments); err != nil {
			logger.Error(err, "failed to unmarshall binding arguments")
			return nil, err
		}
	}
	var bindingInfos []rabbithole.BindingInfo
	var err error
	if binding.Spec.DestinationType == "queue" {
		bindingInfos, err = client.ListQueueBindingsBetween(binding.Spec.Vhost, binding.Spec.Source, binding.Spec.Destination)
	} else {
		bindingInfos, err = client.ListExchangeBindingsBetween(binding.Spec.Vhost, binding.Spec.Source, binding.Spec.Destination)
	}
	if err != nil {
		logger.Error(err, "failed to list binding infos")
		return nil, err
	}
	var info *rabbithole.BindingInfo
	for _, b := range bindingInfos {
		if binding.Spec.RoutingKey == b.RoutingKey && reflect.DeepEqual(b.Arguments, arguments) {
			info = &b
		}
	}
	return info, nil
}
