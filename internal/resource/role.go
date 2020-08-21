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
	"github.com/rabbitmq/cluster-operator/internal/metadata"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	roleName = "peer-discovery"
)

type RoleBuilder struct {
	Instance *rabbitmqv1beta1.RabbitmqCluster
}

func (builder *RabbitmqResourceBuilder) Role() *RoleBuilder {
	return &RoleBuilder{
		Instance: builder.Instance,
	}
}

func (builder *RoleBuilder) UpdateRequiresStsRestart() bool {
	return false
}

func (builder *RoleBuilder) Update(object runtime.Object) error {
	role := object.(*rbacv1.Role)
	role.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)
	role.Annotations = metadata.ReconcileAndFilterAnnotations(role.GetAnnotations(), builder.Instance.Annotations)
	role.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"endpoints"},
			Verbs:     []string{"get"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"create"},
		},
	}

	return nil
}

func (builder *RoleBuilder) Build() (runtime.Object, error) {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: builder.Instance.Namespace,
			Name:      builder.Instance.ChildResourceName(roleName),
		},
	}, nil
}
