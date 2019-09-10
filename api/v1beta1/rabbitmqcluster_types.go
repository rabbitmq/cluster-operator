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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RabbitmqClusterSpec defines the desired state of RabbitmqCluster
type RabbitmqClusterSpec struct {
	// +kubebuilder:validation:Enum=1;3
	Replicas        int                            `json:"replicas"`
	Image           RabbitmqClusterImageSpec       `json:"image,omitempty"`
	ImagePullSecret string                         `json:"imagePullSecret,omitempty"`
	Service         RabbitmqClusterServiceSpec     `json:"service,omitempty"`
	Persistence     RabbitmqClusterPersistenceSpec `json:"persistence,omitempty"`
}

type RabbitmqClusterImageSpec struct {
	Repository string `json:"repository"`
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
}

// +kubebuilder:object:root=true

// RabbitmqCluster is the Schema for the rabbitmqclusters API
type RabbitmqCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RabbitmqClusterSpec   `json:"spec,omitempty"`
	Status RabbitmqClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RabbitmqClusterList contains a list of RabbitmqCluster
type RabbitmqClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RabbitmqCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RabbitmqCluster{}, &RabbitmqClusterList{})
}
