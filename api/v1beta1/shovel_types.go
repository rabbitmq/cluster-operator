package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ShovelSpec defines the desired state of Shovel
// For how to configure Shovel, see: https://www.rabbitmq.com/shovel.html.
type ShovelSpec struct {
	// Required property; cannot be updated
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Default to vhost '/'; cannot be updated
	// +kubebuilder:default:=/
	Vhost string `json:"vhost,omitempty"`
	// Reference to the RabbitmqCluster that this Shovel will be created in.
	// Required property.
	// +kubebuilder:validation:Required
	RabbitmqClusterReference RabbitmqClusterReference `json:"rabbitmqClusterReference"`
	// Secret contains the AMQP URI(s) to configure Shovel destination and source.
	// The Secret must contain the key `destUri` and `srcUri` or operator will error.
	// Both fields should be one or multiple uris separated by ','.
	// Required property.
	// +kubebuilder:validation:Required
	UriSecret *corev1.LocalObjectReference `json:"uriSecret"`
	// +kubebuilder:validation:Enum=on-confirm;on-publish;no-ack
	AckMode                          string `json:"ackMode,omitempty"`
	AddForwardHeaders                bool   `json:"addForwardHeaders,omitempty"`
	DeleteAfter                      string `json:"deleteAfter,omitempty"`
	DestinationAddForwardHeaders     bool   `json:"destAddForwardHeaders,omitempty"`
	DestinationAddTimestampHeader    bool   `json:"destAddTimestampHeader,omitempty"`
	DestinationAddress               string `json:"destAddress,omitempty"`
	DestinationApplicationProperties string `json:"destApplicationProperties,omitempty"`
	DestinationExchange              string `json:"destExchange,omitempty"`
	DestinationExchangeKey           string `json:"destExchangeKey,omitempty"`
	DestinationProperties            string `json:"destProperties,omitempty"`
	DestinationProtocol              string `json:"destProtocol,omitempty"`
	DestinationPublishProperties     string `json:"destPublishProperties,omitempty"`
	DestinationQueue                 string `json:"destQueue,omitempty"`
	PrefetchCount                    int    `json:"prefetchCount,omitempty"`
	ReconnectDelay                   int    `json:"reconnectDelay,omitempty"`
	SourceAddress                    string `json:"srcAddress,omitempty"`
	SourceDeleteAfter                string `json:"srcDeleteAfter,omitempty"`
	SourceExchange                   string `json:"srcExchange,omitempty"`
	SourceExchangeKey                string `json:"srcExchangeKey,omitempty"`
	SourcePrefetchCount              int    `json:"srcPrefetchCount,omitempty"`
	SourceProtocol                   string `json:"srcProtocol,omitempty"`
	SourceQueue                      string `json:"srcQueue,omitempty"`
}

// ShovelStatus defines the observed state of Shovel
type ShovelStatus struct {
	// observedGeneration is the most recent successful generation observed for this Shovel. It corresponds to the
	// Shovel's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
	Conditions         []Condition `json:"conditions,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=all;rabbitmq
// +kubebuilder:subresource:status

// Shovel is the Schema for the shovels API
type Shovel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ShovelSpec   `json:"spec,omitempty"`
	Status ShovelStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ShovelList contains a list of Shovel
type ShovelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Shovel `json:"items"`
}

func (s *Shovel) GroupResource() schema.GroupResource {
	return schema.GroupResource{
		Group:    s.GroupVersionKind().Group,
		Resource: s.GroupVersionKind().Kind,
	}
}

func (s *Shovel) RabbitReference() RabbitmqClusterReference {
	return s.Spec.RabbitmqClusterReference
}

func (s *Shovel) SetStatusConditions(c []Condition) {
	s.Status.Conditions = c
}

func init() {
	SchemeBuilder.Register(&Shovel{}, &ShovelList{})
}
