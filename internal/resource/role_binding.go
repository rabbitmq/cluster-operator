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
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/rabbitmq/cluster-operator/internal/metadata"
)

const (
	roleBindingName = "server"
)

type RoleBindingBuilder struct {
	*RabbitmqResourceBuilder
}

func (builder *RabbitmqResourceBuilder) RoleBinding() *RoleBindingBuilder {
	return &RoleBindingBuilder{builder}
}

func (builder *RoleBindingBuilder) UpdateMayRequireStsRecreate() bool {
	return false
}

func (builder *RoleBindingBuilder) Update(object client.Object) error {
	roleBinding := object.(*rbacv1.RoleBinding)
	roleBinding.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)
	roleBinding.Annotations = metadata.ReconcileAndFilterAnnotations(roleBinding.GetAnnotations(), builder.Instance.Annotations)
	roleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "Role",
		Name:     builder.Instance.ChildResourceName(roleName),
	}
	roleBinding.Subjects = []rbacv1.Subject{
		{
			Kind: "ServiceAccount",
			Name: builder.Instance.ChildResourceName(serviceAccountName),
		},
	}

	if err := controllerutil.SetControllerReference(builder.Instance, roleBinding, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %w", err)
	}
	return nil
}

func (builder *RoleBindingBuilder) Build() (client.Object, error) {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: builder.Instance.Namespace,
			Name:      builder.Instance.ChildResourceName(roleBindingName),
		},
	}, nil
}
