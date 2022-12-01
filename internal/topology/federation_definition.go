/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package internal

import (
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"strings"
)

func GenerateFederationDefinition(f *rabbitmqv1beta1.Federation, uri string) rabbithole.FederationDefinition {
	return rabbithole.FederationDefinition{
		Uri:            strings.Split(uri, ","),
		Expires:        f.Spec.Expires,
		MessageTTL:     int32(f.Spec.MessageTTL),
		MaxHops:        f.Spec.MaxHops,
		PrefetchCount:  f.Spec.PrefetchCount,
		ReconnectDelay: f.Spec.ReconnectDelay,
		AckMode:        f.Spec.AckMode,
		TrustUserId:    f.Spec.TrustUserId,
		Exchange:       f.Spec.Exchange,
		Queue:          f.Spec.Queue,
	}
}
