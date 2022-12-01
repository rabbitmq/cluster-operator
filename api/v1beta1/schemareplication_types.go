/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemaReplicationSpec defines the desired state of SchemaReplication
type SchemaReplicationSpec struct {
	// Reference to the RabbitmqCluster that schema replication would be set for. Must be an existing cluster.
	// +kubebuilder:validation:Required
	RabbitmqClusterReference RabbitmqClusterReference `json:"rabbitmqClusterReference"`
	// Defines a Secret which contains credentials to be used for schema replication.
	// The Secret must contain the keys `username` and `password` in its Data field, or operator will error.
	// Have to set either secretBackend.vault.secretPath or spec.upstreamSecret, but not both.
	// +kubebuilder:validation:Optional
	UpstreamSecret *corev1.LocalObjectReference `json:"upstreamSecret,omitempty"`
	// endpoints should be one or multiple endpoints separated by ','.
	// Must provide either spec.endpoints or endpoints in spec.upstreamSecret.
	// When endpoints are provided in both spec.endpoints and spec.upstreamSecret, spec.endpoints takes
	// precedence.
	Endpoints string `json:"endpoints,omitempty"`
	// Set to fetch user credentials from K8s external secret stores to be used for schema replication.
	SecretBackend SchemaReplicationSecretBackend `json:"secretBackend,omitempty"`
}

// SchemaReplicationSecretBackend configures a single secret backend.
// Today, only Vault exists as supported secret backend.
type SchemaReplicationSecretBackend struct {
	Vault *SchemaReplicationVaultSpec `json:"vault,omitempty"`
}

type SchemaReplicationVaultSpec struct {
	// Path in Vault to access a KV (Key-Value) secret with the fields username and password to be used for replication.
	// For example "secret/data/rabbitmq/config".
	// Optional; if not provided, username and password will come from upstreamSecret instead.
	// Have to set either secretBackend.vault.secretPath or upstreamSecret, but not both.
	SecretPath string `json:"secretPath,omitempty"`
}

// SchemaReplicationStatus defines the observed state of SchemaReplication
type SchemaReplicationStatus struct {
	// observedGeneration is the most recent successful generation observed for this Queue. It corresponds to the
	// Queue's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
	Conditions         []Condition `json:"conditions,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SchemaReplication is the Schema for the schemareplications API
// This feature requires Tanzu RabbitMQ with schema replication plugin.
// For more information, see: https://tanzu.vmware.com/rabbitmq and https://www.rabbitmq.com/definitions-standby.html.
type SchemaReplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SchemaReplicationSpec   `json:"spec,omitempty"`
	Status SchemaReplicationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SchemaReplicationList contains a list of SchemaReplication
type SchemaReplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SchemaReplication `json:"items"`
}

func (s *SchemaReplication) GroupResource() schema.GroupResource {
	return schema.GroupResource{
		Group:    s.GroupVersionKind().Group,
		Resource: s.GroupVersionKind().Kind,
	}
}

func (s *SchemaReplication) RabbitReference() RabbitmqClusterReference {
	return s.Spec.RabbitmqClusterReference
}

func (s *SchemaReplication) SetStatusConditions(c []Condition) {
	s.Status.Conditions = c
}

func init() {
	SchemeBuilder.Register(&SchemaReplication{}, &SchemaReplicationList{})
}
