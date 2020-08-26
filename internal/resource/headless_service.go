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

func (builder *HeadlessServiceBuilder) UpdateRequiresStsRestart() bool {
	return false
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
			{
				Protocol: corev1.ProtocolTCP,
				Port:     25672,
				Name:     "cluster-links", // aka distribution port
			},
		},
		PublishNotReadyAddresses: true,
	}

	return nil
}
