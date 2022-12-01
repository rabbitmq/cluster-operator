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

// using runtime.RawExtension to represent queue arguments
// interface{} is not currently supported by controller runtime
// recommendation is to use json.RawMessage or runtime.RawExtension to represent interface{}

// QueueSpec defines the desired state of Queue
type QueueSpec struct {
	// Name of the queue; required property.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Default to vhost '/'
	// +kubebuilder:default:=/
	Vhost string `json:"vhost,omitempty"`
	Type  string `json:"type,omitempty"`
	// When set to false queues does not survive server restart.
	Durable bool `json:"durable,omitempty"`
	// when set to true, queues that have had at least one consumer before are deleted after the last consumer unsubscribes.
	AutoDelete bool `json:"autoDelete,omitempty"`
	// Queue arguments in the format of KEY: VALUE. e.g. x-delivery-limit: 10000.
	// Configuring queues through arguments is not recommended because they cannot be updated once set; we recommend configuring queues through policies instead.
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	Arguments *runtime.RawExtension `json:"arguments,omitempty"`
	// Reference to the RabbitmqCluster that the queue will be created in.
	// Required property.
	// +kubebuilder:validation:Required
	RabbitmqClusterReference RabbitmqClusterReference `json:"rabbitmqClusterReference"`
}

// QueueStatus defines the observed state of Queue
type QueueStatus struct {
	// observedGeneration is the most recent successful generation observed for this Queue. It corresponds to the
	// Queue's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
	Conditions         []Condition `json:"conditions,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=all;rabbitmq
// +kubebuilder:subresource:status

// Queue is the Schema for the queues API
type Queue struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QueueSpec   `json:"spec,omitempty"`
	Status QueueStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// QueueList contains a list of Queue
type QueueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Queue `json:"items"`
}

func (q *Queue) GroupResource() schema.GroupResource {
	return schema.GroupResource{
		Group:    q.GroupVersionKind().Group,
		Resource: q.GroupVersionKind().Kind,
	}
}

func (q *Queue) RabbitReference() RabbitmqClusterReference {
	return q.Spec.RabbitmqClusterReference
}

func (q *Queue) SetStatusConditions(c []Condition) {
	q.Status.Conditions = c
}

func init() {
	SchemeBuilder.Register(&Queue{}, &QueueList{})
}
