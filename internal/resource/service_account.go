// Copyright (c) 2020 VMware, Inc. or its affiliates.  All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package resource

import (
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	serviceAccountName = "server"
)

type ServiceAccountBuilder struct {
	Instance *rabbitmqv1beta1.RabbitmqCluster
}

func (builder *RabbitmqResourceBuilder) ServiceAccount() *ServiceAccountBuilder {
	return &ServiceAccountBuilder{
		Instance: builder.Instance,
	}
}

func (builder *ServiceAccountBuilder) Update(object runtime.Object) error {
	serviceAccount := object.(*corev1.ServiceAccount)
	serviceAccount.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)
	serviceAccount.Annotations = metadata.ReconcileAndFilterAnnotations(serviceAccount.GetAnnotations(), builder.Instance.Annotations)
	return nil
}

func (builder *ServiceAccountBuilder) Build() (runtime.Object, error) {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: builder.Instance.Namespace,
			Name:      builder.Instance.ChildResourceName(serviceAccountName),
		},
	}, nil
}
