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
)

// generates rabbithole.QueueSettings for a given Queue
// queue.Spec.Arguments (type k8s runtime.RawExtensions) is unmarshalled
// Unmarshall stores float64, for JSON numbers
// See: https://golang.org/pkg/encoding/json/#Unmarshal
func GenerateQueueSettings(q *rabbitmqv1beta1.Queue) (*rabbithole.QueueSettings, error) {
	arguments := make(map[string]interface{})
	if q.Spec.Arguments != nil {
		if err := json.Unmarshal(q.Spec.Arguments.Raw, &arguments); err != nil {
			return nil, fmt.Errorf("failed to unmarshall queue arguments: %v", err)
		}
	}

	return &rabbithole.QueueSettings{
		Type:       q.Spec.Type,
		Durable:    q.Spec.Durable,
		AutoDelete: q.Spec.AutoDelete,
		Arguments:  arguments,
	}, nil
}
