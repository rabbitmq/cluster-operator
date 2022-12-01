package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type RabbitmqClusterReference struct {
	// The name of the RabbitMQ cluster to reference.
	// Have to set either name or connectionSecret, but not both.
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`
	// The namespace of the RabbitMQ cluster to reference.
	// Defaults to the namespace of the requested resource if omitted.
	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace,omitempty"`
	// Secret contains the http management uri for the RabbitMQ cluster.
	// The Secret must contain the key `uri`, `username` and `password` or operator will error.
	// Have to set either name or connectionSecret, but not both.
	// +kubebuilder:validation:Optional
	ConnectionSecret *corev1.LocalObjectReference `json:"connectionSecret,omitempty"`
}

func (r *RabbitmqClusterReference) Matches(new *RabbitmqClusterReference) bool {
	if new.Name != r.Name || new.Namespace != r.Namespace {
		return false
	}

	// when connectionSecret has been updated
	if new.ConnectionSecret != nil && r.ConnectionSecret != nil && *new.ConnectionSecret != *r.ConnectionSecret {
		return false
	}

	// when connectionSecret is removed
	if new.ConnectionSecret == nil && r.ConnectionSecret != nil {
		return false
	}

	// when connectionSecret is added
	if new.ConnectionSecret != nil && r.ConnectionSecret == nil {
		return false
	}

	return true
}

// ValidateOnCreate validates RabbitmqClusterReference on resources create
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both; else it errors
func (ref *RabbitmqClusterReference) ValidateOnCreate(groupResource schema.GroupResource, name string) error {
	if ref.Name != "" && ref.ConnectionSecret != nil {
		return apierrors.NewForbidden(groupResource, name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"),
				"do not provide both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret"))
	}

	if ref.Name == "" && ref.ConnectionSecret == nil {
		return apierrors.NewForbidden(groupResource, name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"),
				"must provide either spec.rabbitmqClusterReference.name or spec.rabbitmqClusterReference.connectionSecret"))
	}
	return nil
}
