package v1beta1

import (
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (s *SchemaReplication) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(s).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-schemareplication,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=schemareplications,versions=v1beta1,name=vschemareplication.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.Validator = &SchemaReplication{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
// either secretBackend.vault.secretPath or upstreamSecret must be provided but not both.
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both.
func (s *SchemaReplication) ValidateCreate() error {
	if err := s.validateSecret(); err != nil {
		return err
	}
	return s.Spec.RabbitmqClusterReference.ValidateOnCreate(s.GroupResource(), s.Name)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
// either secretBackend.vault.secretPath or upstreamSecret must be provided but not both.
func (s *SchemaReplication) ValidateUpdate(old runtime.Object) error {
	oldReplication, ok := old.(*SchemaReplication)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a schema replication type but got a %T", old))
	}

	if !oldReplication.Spec.RabbitmqClusterReference.Matches(&s.Spec.RabbitmqClusterReference) {
		return apierrors.NewForbidden(s.GroupResource(), s.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), "update on rabbitmqClusterReference is forbidden"))
	}
	return s.validateSecret()
}

// ValidateDelete no validation on delete
func (s *SchemaReplication) ValidateDelete() error {
	return nil
}

func (s *SchemaReplication) validateSecret() error {
	if s.Spec.UpstreamSecret != nil && s.Spec.UpstreamSecret.Name != "" && s.Spec.SecretBackend.Vault != nil && s.Spec.SecretBackend.Vault.SecretPath != "" {
		return apierrors.NewForbidden(s.GroupResource(), s.Name,
			field.Forbidden(field.NewPath("spec"),
				"do not provide both secretBackend.vault.secretPath and upstreamSecret"))
	}

	if (s.Spec.UpstreamSecret == nil || s.Spec.UpstreamSecret.Name == "") && (s.Spec.SecretBackend.Vault == nil || s.Spec.SecretBackend.Vault.SecretPath == "") {
		return apierrors.NewForbidden(s.GroupResource(), s.Name,
			field.Forbidden(field.NewPath("spec"),
				"must provide either secretBackend.vault.secretPath or upstreamSecret"))
	}
	return nil
}
