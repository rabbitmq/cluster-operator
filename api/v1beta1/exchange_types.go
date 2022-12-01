/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ExchangeSpec defines the desired state of Exchange
type ExchangeSpec struct {
	// Required property; cannot be updated
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Default to vhost '/'; cannot be updated
	// +kubebuilder:default:=/
	Vhost string `json:"vhost,omitempty"`
	// Cannot be updated
	// +kubebuilder:default:=direct
	Type string `json:"type,omitempty"`
	// Cannot be updated
	Durable bool `json:"durable,omitempty"`
	// Cannot be updated
	AutoDelete bool `json:"autoDelete,omitempty"`
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	Arguments *runtime.RawExtension `json:"arguments,omitempty"`
	// Reference to the RabbitmqCluster that the exchange will be created in.
	// Required property.
	// +kubebuilder:validation:Required
	RabbitmqClusterReference RabbitmqClusterReference `json:"rabbitmqClusterReference"`
}

// ExchangeStatus defines the observed state of Exchange
type ExchangeStatus struct {
	// observedGeneration is the most recent successful generation observed for this Exchange. It corresponds to the
	// Exchange's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
	Conditions         []Condition `json:"conditions,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=all;rabbitmq
// +kubebuilder:subresource:status

// Exchange is the Schema for the exchanges API
type Exchange struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExchangeSpec   `json:"spec,omitempty"`
	Status ExchangeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ExchangeList contains a list of Exchange
type ExchangeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Exchange `json:"items"`
}

func (e *Exchange) GroupResource() schema.GroupResource {
	return schema.GroupResource{
		Group:    e.GroupVersionKind().Group,
		Resource: e.GroupVersionKind().Kind,
	}
}

func (e *Exchange) RabbitReference() RabbitmqClusterReference {
	return e.Spec.RabbitmqClusterReference
}

func (e *Exchange) SetStatusConditions(c []Condition) {
	e.Status.Conditions = c
}

func init() {
	SchemeBuilder.Register(&Exchange{}, &ExchangeList{})
}
