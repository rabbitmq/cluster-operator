// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
//

package resource

import (
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

type RabbitmqResourceBuilder struct {
	Instance *rabbitmqv1beta1.RabbitmqCluster
	Scheme   *runtime.Scheme
}

type ResourceBuilder interface {
	Update(runtime.Object) error
	Build() (runtime.Object, error)
	UpdateRequiresStsRestart() bool
}

func (builder *RabbitmqResourceBuilder) ResourceBuilders() ([]ResourceBuilder, error) {
	return []ResourceBuilder{
		builder.HeadlessService(),
		builder.ClientService(),
		builder.ErlangCookie(),
		builder.AdminSecret(),
		builder.RabbitmqPluginsConfigMap(),
		builder.ServerConfigMap(),
		builder.ServiceAccount(),
		builder.Role(),
		builder.RoleBinding(),
		builder.StatefulSet(),
	}, nil
}
