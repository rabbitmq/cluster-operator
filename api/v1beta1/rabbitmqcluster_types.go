/*
Copyright 2019 Pivotal.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RabbitmqClusterSpec defines the desired state of RabbitmqCluster
type RabbitmqClusterSpec struct {
	// +kubebuilder:validation:Enum=1;3
	Replicas        int                            `json:"replicas"`
	Image           string                         `json:"image,omitempty"`
	ImagePullSecret string                         `json:"imagePullSecret,omitempty"`
	Service         RabbitmqClusterServiceSpec     `json:"service,omitempty"`
	Persistence     RabbitmqClusterPersistenceSpec `json:"persistence,omitempty"`
	Resource        RabbitmqClusterResourceSpec    `json:"resource,omitempty"`
	Affinity        *corev1.Affinity               `json:"affinity,omitempty"`
}

type RabbitmqClusterResourceSpec struct {
	Request RabbitmqClusterComputeResource `json:"request,omitempty"`
	Limit   RabbitmqClusterComputeResource `json:"limit,omitempty"`
}

type RabbitmqClusterPersistenceSpec struct {
	StorageClassName string `json:"storageClassName,omitempty"`
	Storage          string `json:"storage,omitempty"`
}

type RabbitmqClusterServiceSpec struct {
	// +kubebuilder:validation:Enum=ClusterIP;LoadBalancer;NodePort
	Type        string            `json:"type,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// RabbitmqClusterStatus defines the observed state of RabbitmqCluster
type RabbitmqClusterStatus struct {
	ClusterStatus string `json:"clusterStatus,omitempty"`
}

// +kubebuilder:object:root=true

// RabbitmqCluster is the Schema for the rabbitmqclusters API
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.clusterStatus"
type RabbitmqCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RabbitmqClusterSpec   `json:"spec,omitempty"`
	Status RabbitmqClusterStatus `json:"status,omitempty"`
}

type RabbitmqClusterComputeResource struct {
	// +kubebuilder:validation:Pattern="^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$"
	CPU string `json:"cpu,omitempty"`
	// +kubebuilder:validation:Pattern="^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$"
	Memory string `json:"memory,omitempty"`
}

// +kubebuilder:object:root=true

// RabbitmqClusterList contains a list of RabbitmqCluster
type RabbitmqClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RabbitmqCluster `json:"items"`
}

func (r RabbitmqCluster) ChildResourceName(name string) string {
	return strings.Join([]string{r.Name, "rabbitmq", name}, "-")
}

func init() {
	SchemeBuilder.Register(&RabbitmqCluster{}, &RabbitmqClusterList{})
}
