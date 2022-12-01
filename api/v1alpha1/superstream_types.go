/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package v1alpha1

import (
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SuperStreamSpec defines the desired state of SuperStream
type SuperStreamSpec struct {
	// Name of the queue; required property.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Default to vhost '/'; cannot be updated
	// +kubebuilder:default:=/
	Vhost string `json:"vhost,omitempty"`
	// Number of partitions to create within this super stream.
	// Defaults to '3'.
	// +kubebuilder:default:=3
	Partitions int `json:"partitions,omitempty"`
	// Routing keys to use for each of the partitions in the SuperStream
	// If unset, the routing keys for the partitions will be set to the index of the partitions
	// +kubebuilder:validation:Optional
	RoutingKeys []string `json:"routingKeys,omitempty"`
	// Reference to the RabbitmqCluster that the SuperStream will be created in.
	// Required property.
	// +kubebuilder:validation:Required
	RabbitmqClusterReference rabbitmqv1beta1.RabbitmqClusterReference `json:"rabbitmqClusterReference"`
}

// SuperStreamStatus defines the observed state of SuperStream
type SuperStreamStatus struct {
	// observedGeneration is the most recent successful generation observed for this SuperStream. It corresponds to the
	// SuperStream's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64                       `json:"observedGeneration,omitempty"`
	Conditions         []rabbitmqv1beta1.Condition `json:"conditions,omitempty"`
	// Partitions are a list of the stream queue names which form the partitions of this SuperStream.
	Partitions []string `json:"partitions,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=all;rabbitmq
// +kubebuilder:subresource:status

// SuperStream is the Schema for the queues API
type SuperStream struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SuperStreamSpec   `json:"spec,omitempty"`
	Status SuperStreamStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SuperStreamList contains a list of SuperStreams
type SuperStreamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SuperStream `json:"items"`
}

func (q *SuperStream) GroupResource() schema.GroupResource {
	return schema.GroupResource{
		Group:    q.GroupVersionKind().Group,
		Resource: q.GroupVersionKind().Kind,
	}
}

func init() {
	SchemeBuilder.Register(&SuperStream{}, &SuperStreamList{})
}
