/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package internal

import (
	"encoding/json"
	"fmt"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"strings"
)

func GenerateBindingInfo(binding *rabbitmqv1beta1.Binding) (*rabbithole.BindingInfo, error) {
	arguments := make(map[string]interface{})
	if binding.Spec.Arguments != nil {
		if err := json.Unmarshal(binding.Spec.Arguments.Raw, &arguments); err != nil {
			return nil, fmt.Errorf("failed to unmarshall binding arguments: %v", err)
		}
	}

	return &rabbithole.BindingInfo{
		Vhost:           binding.Spec.Vhost,
		Source:          binding.Spec.Source,
		Destination:     binding.Spec.Destination,
		DestinationType: binding.Spec.DestinationType,
		RoutingKey:      binding.Spec.RoutingKey,
		Arguments:       arguments,
	}, nil
}

// Generate binding properties key which is necessary when deleting a binding
// Binding properties key is:
// when routing key and argument are not provided, properties key is "~"
// when routing key is set and no argument is provided, properties key is the routing key itself
// if routing key has character '~', it's replaced by '%7E'
// when arguments are provided, properties key is the routing key (could be empty) plus the hash of arguments
// the hash function used is 'erlang:phash2' and it's erlang specific; GeneratePropertiesKey returns empty
// string if arguments are provided (deletion not supported)

func GeneratePropertiesKey(binding *rabbitmqv1beta1.Binding) string {
	if binding.Spec.RoutingKey == "" {
		return "~"
	}
	if binding.Spec.Arguments == nil {
		return strings.ReplaceAll(binding.Spec.RoutingKey, "~", "%7E")
	}

	return ""
}
