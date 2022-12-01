package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// FederationSpec defines the desired state of Federation
// For how to configure federation upstreams, see: https://www.rabbitmq.com/federation-reference.html.
type FederationSpec struct {
	// Required property; cannot be updated
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Default to vhost '/'; cannot be updated
	// +kubebuilder:default:=/
	Vhost string `json:"vhost,omitempty"`
	// Reference to the RabbitmqCluster that this federation upstream will be created in.
	// Required property.
	// +kubebuilder:validation:Required
	RabbitmqClusterReference RabbitmqClusterReference `json:"rabbitmqClusterReference"`
	// Secret contains the AMQP URI(s) for the upstream.
	// The Secret must contain the key `uri` or operator will error.
	// `uri` should be one or multiple uris separated by ','.
	// Required property.
	// +kubebuilder:validation:Required
	UriSecret     *corev1.LocalObjectReference `json:"uriSecret"`
	PrefetchCount int                          `json:"prefetch-count,omitempty"`
	// +kubebuilder:validation:Enum=on-confirm;on-publish;no-ack
	AckMode        string `json:"ackMode,omitempty"`
	Expires        int    `json:"expires,omitempty"`
	MessageTTL     int    `json:"messageTTL,omitempty"`
	MaxHops        int    `json:"maxHops,omitempty"`
	ReconnectDelay int    `json:"reconnectDelay,omitempty"`
	TrustUserId    bool   `json:"trustUserId,omitempty"`
	Exchange       string `json:"exchange,omitempty"`
	Queue          string `json:"queue,omitempty"`
}

// FederationStatus defines the observed state of Federation
type FederationStatus struct {
	// observedGeneration is the most recent successful generation observed for this Federation. It corresponds to the
	// Federation's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
	Conditions         []Condition `json:"conditions,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=all;rabbitmq
// +kubebuilder:subresource:status

// Federation is the Schema for the federations API
type Federation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FederationSpec   `json:"spec,omitempty"`
	Status FederationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FederationList contains a list of Federation
type FederationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Federation `json:"items"`
}

func (f *Federation) GroupResource() schema.GroupResource {
	return schema.GroupResource{
		Group:    f.GroupVersionKind().Group,
		Resource: f.GroupVersionKind().Kind,
	}
}

func (f *Federation) RabbitReference() RabbitmqClusterReference {
	return f.Spec.RabbitmqClusterReference
}

func (f *Federation) SetStatusConditions(c []Condition) {
	f.Status.Conditions = c
}

func init() {
	SchemeBuilder.Register(&Federation{}, &FederationList{})
}
