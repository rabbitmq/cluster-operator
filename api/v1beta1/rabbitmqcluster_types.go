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
	"math"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	rabbitmqImage             string             = "rabbitmq:3.8.2"
	defaultPersistentCapacity int64              = 10
	defaultMemoryLimit        string             = "2Gi"
	defaultCPULimit           string             = "2000m"
	defaultMemoryRequest      string             = "2Gi"
	defaultCPURequest         string             = "1000m"
	defaultServiceType        corev1.ServiceType = corev1.ServiceTypeClusterIP
)

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

// RabbitmqClusterSpec defines the desired state of RabbitmqCluster
type RabbitmqClusterSpec struct {
	// +kubebuilder:validation:Enum=1;3
	Replicas        int32                          `json:"replicas"`
	Image           string                         `json:"image,omitempty"`
	ImagePullSecret string                         `json:"imagePullSecret,omitempty"`
	Service         RabbitmqClusterServiceSpec     `json:"service,omitempty"`
	Persistence     RabbitmqClusterPersistenceSpec `json:"persistence,omitempty"`
	Resources       *corev1.ResourceRequirements   `json:"resources,omitempty"`
	Affinity        *corev1.Affinity               `json:"affinity,omitempty"`
	Tolerations     []corev1.Toleration            `json:"tolerations,omitempty"`
}

type RabbitmqClusterPersistenceSpec struct {
	StorageClassName *string               `json:"storageClassName,omitempty"`
	Storage          *k8sresource.Quantity `json:"storage,omitempty"`
}

type RabbitmqClusterServiceSpec struct {
	// +kubebuilder:validation:Enum=ClusterIP;LoadBalancer;NodePort
	Type        corev1.ServiceType `json:"type,omitempty"`
	Annotations map[string]string  `json:"annotations,omitempty"`
}

// RabbitmqClusterStatus defines the observed state of RabbitmqCluster
type RabbitmqClusterStatus struct {
	ClusterStatus string `json:"clusterStatus,omitempty"`
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

var RabbitmqClusterDefaults RabbitmqCluster = RabbitmqCluster{
	Spec: RabbitmqClusterSpec{
		Replicas: 1,
		Image:    rabbitmqImage,
		Service: RabbitmqClusterServiceSpec{
			Type: defaultServiceType,
		},
		Persistence: RabbitmqClusterPersistenceSpec{
			Storage: k8sresource.NewQuantity(defaultPersistentCapacity*int64(math.Pow(2, 30)), k8sresource.BinarySI),
		},
		Resources: &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]k8sresource.Quantity{
				"cpu":    k8sresource.MustParse(defaultCPULimit),
				"memory": k8sresource.MustParse(defaultMemoryLimit),
			},
			Requests: map[corev1.ResourceName]k8sresource.Quantity{
				"cpu":    k8sresource.MustParse(defaultCPURequest),
				"memory": k8sresource.MustParse(defaultMemoryRequest),
			},
		},
	},
}

func MergeDefaults(current, template RabbitmqCluster) *RabbitmqCluster {
	var mergedRabbitmq RabbitmqCluster = current

	emptyRabbitmq := RabbitmqCluster{}
	// Note: we do not check for ImagePullSecret or StorageClassName since the default and nil value are both "".
	// The logic of the check would be 'if actual is an empty string, then set to an empty string'
	// We also do not check for Annotations as the nil value will be the empty map.

	if mergedRabbitmq.Spec.Replicas == emptyRabbitmq.Spec.Replicas {
		mergedRabbitmq.Spec.Replicas = template.Spec.Replicas
	}

	if mergedRabbitmq.Spec.Image == emptyRabbitmq.Spec.Image {
		mergedRabbitmq.Spec.Image = template.Spec.Image
	}

	if mergedRabbitmq.Spec.Service.Type == emptyRabbitmq.Spec.Service.Type {
		mergedRabbitmq.Spec.Service.Type = template.Spec.Service.Type
	}

	if reflect.DeepEqual(mergedRabbitmq.Spec.Persistence.Storage, emptyRabbitmq.Spec.Persistence.Storage) {
		mergedRabbitmq.Spec.Persistence.Storage = template.Spec.Persistence.Storage
	}

	if reflect.DeepEqual(mergedRabbitmq.Spec.Resources, emptyRabbitmq.Spec.Resources) {
		mergedRabbitmq.Spec.Resources = template.Spec.Resources
	}

	return &mergedRabbitmq
}
