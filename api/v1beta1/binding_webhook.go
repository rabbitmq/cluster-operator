package v1beta1

import (
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (b *Binding) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(b).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-binding,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=bindings,versions=v1beta1,name=vbinding.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.Validator = &Binding{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (b *Binding) ValidateCreate() error {
	return b.Spec.RabbitmqClusterReference.ValidateOnCreate(b.GroupResource(), b.Name)
}

// ValidateUpdate updates on vhost and rabbitmqClusterReference are forbidden
func (b *Binding) ValidateUpdate(old runtime.Object) error {
	oldBinding, ok := old.(*Binding)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a binding but got a %T", old))
	}

	var allErrs field.ErrorList
	detailMsg := "updates on vhost and rabbitmqClusterReference are all forbidden"

	if b.Spec.Vhost != oldBinding.Spec.Vhost {
		return apierrors.NewForbidden(b.GroupResource(), b.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if !oldBinding.Spec.RabbitmqClusterReference.Matches(&b.Spec.RabbitmqClusterReference) {
		return apierrors.NewForbidden(b.GroupResource(), b.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}

	if b.Spec.Source != oldBinding.Spec.Source {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "source"),
			b.Spec.Source,
			"source cannot be updated",
		))
	}

	if b.Spec.Destination != oldBinding.Spec.Destination {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "destination"),
			b.Spec.Destination,
			"destination cannot be updated",
		))
	}

	if b.Spec.DestinationType != oldBinding.Spec.DestinationType {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "destinationType"),
			b.Spec.DestinationType,
			"destinationType cannot be updated",
		))
	}

	if b.Spec.RoutingKey != oldBinding.Spec.RoutingKey {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "routingKey"),
			b.Spec.RoutingKey,
			"routingKey cannot be updated",
		))
	}

	if !reflect.DeepEqual(b.Spec.Arguments, oldBinding.Spec.Arguments) {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "arguments"),
			b.Spec.Arguments,
			"arguments cannot be updated",
		))
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(GroupVersion.WithKind("Binding").GroupKind(), b.Name, allErrs)
}

// no validation logic on delete
func (b *Binding) ValidateDelete() error {
	return nil
}
