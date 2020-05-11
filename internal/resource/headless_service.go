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
	headlessServiceName = "headless"
)

type HeadlessServiceBuilder struct {
	Instance *rabbitmqv1beta1.RabbitmqCluster
}

func (builder *RabbitmqResourceBuilder) HeadlessService() *HeadlessServiceBuilder {
	return &HeadlessServiceBuilder{
		Instance: builder.Instance,
	}
}

func (builder *HeadlessServiceBuilder) Build() (runtime.Object, error) {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName(headlessServiceName),
			Namespace: builder.Instance.Namespace,
		},
	}, nil
}

func (builder *HeadlessServiceBuilder) Update(object runtime.Object) error {
	service := object.(*corev1.Service)
	service.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)
	service.Annotations = metadata.ReconcileAndFilterAnnotations(service.GetAnnotations(), builder.Instance.Annotations)
	service.Spec = corev1.ServiceSpec{
		ClusterIP: "None",
		Selector:  metadata.LabelSelector(builder.Instance.Name),
		Ports: []corev1.ServicePort{
			{
				Protocol: corev1.ProtocolTCP,
				Port:     4369,
				Name:     "epmd",
			},
		},
		PublishNotReadyAddresses: true,
	}

	return nil
}
