// Copyright (c) 2020 VMware, Inc. or its affiliates.  All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package resource

import (
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

type RabbitmqResourceBuilder struct {
	Instance *rabbitmqv1beta1.RabbitmqCluster
	Scheme   *runtime.Scheme
}

type ResourceBuilder interface {
	Update(runtime.Object) error
	Build() (runtime.Object, error)
}

func (builder *RabbitmqResourceBuilder) ResourceBuilders() ([]ResourceBuilder, error) {
	return []ResourceBuilder{
		builder.HeadlessService(),
		builder.IngressService(),
		builder.ErlangCookie(),
		builder.AdminSecret(),
		builder.ServerConfigMap(),
		builder.ServiceAccount(),
		builder.Role(),
		builder.RoleBinding(),
		builder.StatefulSet(),
	}, nil
}
