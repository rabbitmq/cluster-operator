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
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/util/strategicpatch"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (builder *RabbitmqResourceBuilder) ClientService() *ClientServiceBuilder {
	return &ClientServiceBuilder{
		Instance: builder.Instance,
		Scheme:   builder.Scheme,
	}
}

type ClientServiceBuilder struct {
	Instance *rabbitmqv1beta1.RabbitmqCluster
	Scheme   *runtime.Scheme
}

func (builder *ClientServiceBuilder) UpdateRequiresStsRestart() bool {
	return false
}

func (builder *ClientServiceBuilder) Build() (runtime.Object, error) {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName("client"),
			Namespace: builder.Instance.Namespace,
		},
	}, nil
}

func (builder *ClientServiceBuilder) Update(object runtime.Object) error {
	service := object.(*corev1.Service)
	builder.setAnnotations(service)
	service.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)
	service.Spec.Type = builder.Instance.Spec.Service.Type
	service.Spec.Selector = metadata.LabelSelector(builder.Instance.Name)

	service.Spec.Ports = builder.updatePorts(service.Spec.Ports)

	if builder.Instance.Spec.Service.Type == "ClusterIP" || builder.Instance.Spec.Service.Type == "" {
		for i := range service.Spec.Ports {
			service.Spec.Ports[i].NodePort = int32(0)
		}
	}

	if builder.Instance.Spec.Override.ClientService != nil {
		if err := applySvcOverride(service, builder.Instance.Spec.Override.ClientService); err != nil {
			return fmt.Errorf("failed applying Client Service override: %v", err)
		}
	}

	if err := controllerutil.SetControllerReference(builder.Instance, service, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %v", err)
	}

	return nil
}

func applySvcOverride(svc *corev1.Service, override *rabbitmqv1beta1.ClientService) error {
	if override.EmbeddedLabelsAnnotations != nil {
		copyLabelsAnnotations(&svc.ObjectMeta, *override.EmbeddedLabelsAnnotations)
	}

	if override.Spec != nil {
		originalSvcSpec, err := json.Marshal(svc.Spec)
		if err != nil {
			return fmt.Errorf("error marshalling Client ServiceSpec: %v", err)
		}

		patch, err := json.Marshal(override.Spec)
		if err != nil {
			return fmt.Errorf("error marshalling Client ServiceSpec override: %v", err)
		}

		patchedJSON, err := strategicpatch.StrategicMergePatch(originalSvcSpec, patch, corev1.ServiceSpec{})
		if err != nil {
			return fmt.Errorf("error patching CLient ServiceSpec: %v", err)
		}

		patchedSvcSpec := corev1.ServiceSpec{}
		err = json.Unmarshal(patchedJSON, &patchedSvcSpec)
		if err != nil {
			return fmt.Errorf("error unmarshalling patched Client ServiceSpec: %v", err)
		}
		svc.Spec = patchedSvcSpec
	}

	return nil
}

func (builder *ClientServiceBuilder) updatePorts(servicePorts []corev1.ServicePort) []corev1.ServicePort {
	servicePortsMap := map[string]corev1.ServicePort{
		"amqp": {
			Protocol: corev1.ProtocolTCP,
			Port:     5672,
			Name:     "amqp",
		},
		"management": {
			Protocol: corev1.ProtocolTCP,
			Port:     15672,
			Name:     "management",
		},
	}
	if builder.Instance.AdditionalPluginEnabled("rabbitmq_mqtt") {
		servicePortsMap["mqtt"] = corev1.ServicePort{
			Protocol: corev1.ProtocolTCP,
			Port:     1883,
			Name:     "mqtt",
		}
	}
	if builder.Instance.AdditionalPluginEnabled("rabbitmq_web_mqtt") {
		servicePortsMap["web-mqtt"] = corev1.ServicePort{
			Protocol: corev1.ProtocolTCP,
			Port:     15675,
			Name:     "web-mqtt",
		}
	}
	if builder.Instance.AdditionalPluginEnabled("rabbitmq_stomp") {
		servicePortsMap["stomp"] = corev1.ServicePort{
			Protocol: corev1.ProtocolTCP,
			Port:     61613,
			Name:     "stomp",
		}
	}
	if builder.Instance.AdditionalPluginEnabled("rabbitmq_web_stomp") {
		servicePortsMap["web-stomp"] = corev1.ServicePort{
			Protocol: corev1.ProtocolTCP,
			Port:     15674,
			Name:     "web-stomp",
		}
	}
	if builder.Instance.TLSEnabled() {
		servicePortsMap["amqps"] = corev1.ServicePort{
			Protocol: corev1.ProtocolTCP,
			Port:     5671,
			Name:     "amqps",
		}
	}

	var updatedServicePorts []corev1.ServicePort

	for _, servicePort := range servicePorts {
		if value, exists := servicePortsMap[servicePort.Name]; exists {
			value.NodePort = servicePort.NodePort

			updatedServicePorts = append(updatedServicePorts, value)
			delete(servicePortsMap, servicePort.Name)
		}
	}

	for _, value := range servicePortsMap {
		updatedServicePorts = append(updatedServicePorts, value)
	}

	return updatedServicePorts

}

func (builder *ClientServiceBuilder) setAnnotations(service *corev1.Service) {
	if builder.Instance.Spec.Service.Annotations != nil {
		service.Annotations = metadata.ReconcileAnnotations(metadata.ReconcileAndFilterAnnotations(service.Annotations, builder.Instance.Annotations), builder.Instance.Spec.Service.Annotations)
	} else {
		service.Annotations = metadata.ReconcileAndFilterAnnotations(service.Annotations, builder.Instance.Annotations)
	}
}
