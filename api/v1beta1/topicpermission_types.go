package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TopicPermissionSpec defines the desired state of TopicPermission
type TopicPermissionSpec struct {
	// Name of an existing user; must provide user or userReference, else create/update will fail; cannot be updated.
	User string `json:"user,omitempty"`
	// Reference to an existing user.rabbitmq.com object; must provide user or userReference, else create/update will fail; cannot be updated.
	UserReference *corev1.LocalObjectReference `json:"userReference,omitempty"`
	// Name of an existing vhost; required property; cannot be updated.
	// +kubebuilder:validation:Required
	Vhost string `json:"vhost"`
	// Permissions to grant to the user to a topic exchange; required property.
	// +kubebuilder:validation:Required
	Permissions TopicPermissionConfig `json:"permissions"`
	// Reference to the RabbitmqCluster that both the provided user and vhost are.
	// Required property.
	// +kubebuilder:validation:Required
	RabbitmqClusterReference RabbitmqClusterReference `json:"rabbitmqClusterReference"`
}

type TopicPermissionConfig struct {
	// Name of a topic exchange; required property; cannot be updated.
	// +kubebuilder:validation:Required
	Exchange string `json:"exchange,omitempty"`
	// +kubebuilder:validation:Optional
	Read string `json:"read,omitempty"`
	// +kubebuilder:validation:Optional
	Write string `json:"write,omitempty"`
}

// TopicPermissionStatus defines the observed state of TopicPermission
type TopicPermissionStatus struct {
	// observedGeneration is the most recent successful generation observed for this TopicPermission. It corresponds to the
	// TopicPermission's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
	Conditions         []Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TopicPermission is the Schema for the topicpermissions API
type TopicPermission struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TopicPermissionSpec   `json:"spec,omitempty"`
	Status TopicPermissionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TopicPermissionList contains a list of TopicPermission
type TopicPermissionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TopicPermission `json:"items"`
}

func (t *TopicPermission) GroupResource() schema.GroupResource {
	return schema.GroupResource{
		Group:    t.GroupVersionKind().Group,
		Resource: t.GroupVersionKind().Kind,
	}
}

func (t *TopicPermission) RabbitReference() RabbitmqClusterReference {
	return t.Spec.RabbitmqClusterReference
}

func (t *TopicPermission) SetStatusConditions(c []Condition) {
	t.Status.Conditions = c
}

func init() {
	SchemeBuilder.Register(&TopicPermission{}, &TopicPermissionList{})
}
