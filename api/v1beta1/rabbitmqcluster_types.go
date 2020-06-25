// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.

package v1beta1

import (
	"reflect"
	"strings"

	"github.com/pivotal/rabbitmq-for-kubernetes/internal/status"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

const (
	rabbitmqImage             string             = "rabbitmq:3.8.3"
	defaultPersistentCapacity string             = "10Gi"
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

// Spec is the desired state of the RabbitmqCluster Custom Resource.
type RabbitmqClusterSpec struct {
	// Replicas is the number of nodes in the RabbitMQ cluster. Each node is deployed as a Replica in a StatefulSet. Only 1, 3, 5 replicas clusters are tested.
	// +kubebuilder:validation:Minimum:=0
	Replicas *int32 `json:"replicas"`
	// Image is the name of the RabbitMQ docker image to use for RabbitMQ nodes in the RabbitmqCluster.
	Image string `json:"image,omitempty"`
	// Name of the Secret resource containing access credentials to the registry for the RabbitMQ image. Required if the docker registry is private.
	ImagePullSecret string                         `json:"imagePullSecret,omitempty"`
	Service         RabbitmqClusterServiceSpec     `json:"service,omitempty"`
	Persistence     RabbitmqClusterPersistenceSpec `json:"persistence,omitempty"`
	Resources       *corev1.ResourceRequirements   `json:"resources,omitempty"`
	Affinity        *corev1.Affinity               `json:"affinity,omitempty"`
	// Tolerations is the list of Toleration resources attached to each Pod in the RabbitmqCluster.
	Tolerations []corev1.Toleration              `json:"tolerations,omitempty"`
	Rabbitmq    RabbitmqClusterConfigurationSpec `json:"rabbitmq,omitempty"`
	TLS         TLSSpec                          `json:"tls,omitempty"`
}

type TLSSpec struct {
	SecretName   string `json:"secretName,omitempty"`
	CaSecretName string `json:"caSecretName,omitempty"`
	CaCertName   string `json:"caCertName,omitempty"`
}

// kubebuilder validating tags 'Pattern' and 'MaxLength' must be specified on string type.
// Alias type 'string' as 'Plugin' to specify schema validation on items of the list 'AdditionalPlugins'
// +kubebuilder:validation:Pattern:="^\\w+$"
// +kubebuilder:validation:MaxLength=100
type Plugin string

// Rabbitmq related configurations
type RabbitmqClusterConfigurationSpec struct {
	// List of plugins to enable in addition to essential plugins: rabbitmq_management, rabbitmq_prometheus, and rabbitmq_peer_discovery_k8s.
	// +kubebuilder:validation:MaxItems:=100
	AdditionalPlugins []Plugin `json:"additionalPlugins,omitempty"`
	// Modify to add to the rabbitmq.conf file in addition to default configurations set by the operator. Modify this property on an existing RabbitmqCluster will trigger a StatefulSet rolling restart and will cause rabbitmq downtime.
	// +kubebuilder:validation:MaxLength:=2000
	AdditionalConfig string `json:"additionalConfig,omitempty"`
}

// The settings for the persistent storage desired for each Pod in the RabbitmqCluster.
type RabbitmqClusterPersistenceSpec struct {
	// StorageClassName is the name of the StorageClass to claim a PersistentVolume from.
	StorageClassName *string `json:"storageClassName,omitempty"`
	// The requested size of the persistent volume attached to each Pod in the RabbitmqCluster.
	Storage *k8sresource.Quantity `json:"storage,omitempty"`
}

// Settable attributes for the Client Service resource.
type RabbitmqClusterServiceSpec struct {
	// +kubebuilder:validation:Enum=ClusterIP;LoadBalancer;NodePort
	Type corev1.ServiceType `json:"type,omitempty"`
	// Annotations to add to the Client Service.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// Status presents the observed state of RabbitmqCluster
type RabbitmqClusterStatus struct {
	ClusterStatus string `json:"clusterStatus,omitempty"`
	// Set of Conditions describing the current state of the RabbitmqCluster
	Conditions []status.RabbitmqClusterCondition `json:"conditions"`

	// Identifying information on internal resources
	Admin *RabbitmqClusterAdmin `json:"admin,omitempty"`
}

type RabbitmqClusterAdmin struct {
	SecretReference  *RabbitmqClusterSecretReference  `json:"secretReference,omitempty"`
	ServiceReference *RabbitmqClusterServiceReference `json:"serviceReference,omitempty"`
}

type RabbitmqClusterSecretReference struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Keys      map[string]string `json:"keys"`
}

type RabbitmqClusterServiceReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

func (cluster *RabbitmqCluster) TLSEnabled() bool {
	return cluster.Spec.TLS.SecretName != ""
}

func (cluster *RabbitmqCluster) MutualTLSEnabled() bool {
	return cluster.TLSEnabled() && cluster.Spec.TLS.CaSecretName != ""
}

func (cluster *RabbitmqCluster) SingleTLSSecret() bool {
	return cluster.MutualTLSEnabled() && cluster.Spec.TLS.CaSecretName == cluster.Spec.TLS.SecretName
}

func (rmqStatus *RabbitmqClusterStatus) SetConditions(resources []runtime.Object) {
	var existingAllPodsReadyCondition *status.RabbitmqClusterCondition
	var existingClusterAvailableCondition *status.RabbitmqClusterCondition
	var existingNoWarningsCondition *status.RabbitmqClusterCondition
	var existingReconcileCondition *status.RabbitmqClusterCondition

	for _, condition := range rmqStatus.Conditions {
		switch condition.Type {
		case status.AllReplicasReady:
			existingAllPodsReadyCondition = condition.DeepCopy()
		case status.ClusterAvailable:
			existingClusterAvailableCondition = condition.DeepCopy()
		case status.NoWarnings:
			existingNoWarningsCondition = condition.DeepCopy()
		case status.ReconcileSuccess:
			existingReconcileCondition = condition.DeepCopy()
		}
	}

	allReplicasReadyCond := status.AllReplicasReadyCondition(resources, existingAllPodsReadyCondition)
	clusterAvailableCond := status.ClusterAvailableCondition(resources, existingClusterAvailableCondition)
	noWarningsCond := status.NoWarningsCondition(resources, existingNoWarningsCondition)

	var reconciledCondition status.RabbitmqClusterCondition
	if existingReconcileCondition != nil {
		reconciledCondition = *existingReconcileCondition
	} else {
		reconciledCondition = status.ReconcileSuccessCondition(corev1.ConditionUnknown, "Initialising", "")
	}

	currentStatusConditions := []status.RabbitmqClusterCondition{
		allReplicasReadyCond,
		clusterAvailableCond,
		noWarningsCond,
		reconciledCondition,
	}

	rmqStatus.Conditions = currentStatusConditions
}

func (rmqStatus *RabbitmqClusterStatus) SetCondition(condType status.RabbitmqClusterConditionType,
	condStatus corev1.ConditionStatus, reason string, messages ...string) {
	for i := range rmqStatus.Conditions {
		if rmqStatus.Conditions[i].Type == condType {
			rmqStatus.Conditions[i].UpdateState(condStatus)
			rmqStatus.Conditions[i].UpdateReason(reason, messages...)
			break
		}
	}
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

func getDefaultPersistenceStorageQuantity() *k8sresource.Quantity {
	tenGi := k8sresource.MustParse(defaultPersistentCapacity)
	return &tenGi
}

var one int32 = 1

var rabbitmqClusterDefaults = RabbitmqCluster{
	Spec: RabbitmqClusterSpec{
		Replicas: &one,
		Image:    rabbitmqImage,
		Service: RabbitmqClusterServiceSpec{
			Type: defaultServiceType,
		},
		Persistence: RabbitmqClusterPersistenceSpec{
			Storage: getDefaultPersistenceStorageQuantity(),
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

func MergeDefaults(current RabbitmqCluster) *RabbitmqCluster {
	var mergedRabbitmq RabbitmqCluster = current

	emptyRabbitmq := RabbitmqCluster{}
	// Note: we do not check for ImagePullSecret or StorageClassName since the default and nil value are both "".
	// The logic of the check would be 'if actual is an empty string, then set to an empty string'
	// We also do not check for Annotations as the nil value will be the empty map.

	if mergedRabbitmq.Spec.Replicas == emptyRabbitmq.Spec.Replicas {
		mergedRabbitmq.Spec.Replicas = rabbitmqClusterDefaults.Spec.Replicas
	}

	if mergedRabbitmq.Spec.Image == emptyRabbitmq.Spec.Image {
		mergedRabbitmq.Spec.Image = rabbitmqClusterDefaults.Spec.Image
	}

	if mergedRabbitmq.Spec.Service.Type == emptyRabbitmq.Spec.Service.Type {
		mergedRabbitmq.Spec.Service.Type = rabbitmqClusterDefaults.Spec.Service.Type
	}

	if reflect.DeepEqual(mergedRabbitmq.Spec.Persistence.Storage, emptyRabbitmq.Spec.Persistence.Storage) {
		mergedRabbitmq.Spec.Persistence.Storage = rabbitmqClusterDefaults.Spec.Persistence.Storage
	}

	if reflect.DeepEqual(mergedRabbitmq.Spec.Resources, emptyRabbitmq.Spec.Resources) {
		mergedRabbitmq.Spec.Resources = rabbitmqClusterDefaults.Spec.Resources
	}

	return &mergedRabbitmq
}
