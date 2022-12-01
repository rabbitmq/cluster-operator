package v1beta1

import (
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (r *Exchange) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-exchange,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=exchanges,versions=v1beta1,name=vexchange.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.Validator = &Exchange{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (e *Exchange) ValidateCreate() error {
	return e.Spec.RabbitmqClusterReference.ValidateOnCreate(e.GroupResource(), e.Name)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
// returns error type 'forbidden' for updates that the controller chooses to disallow: exchange name/vhost/rabbitmqClusterReference
// returns error type 'invalid' for updates that will be rejected by rabbitmq server: exchange types/autoDelete/durable
// exchange.spec.arguments can be updated
func (e *Exchange) ValidateUpdate(old runtime.Object) error {
	oldExchange, ok := old.(*Exchange)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected an exchange but got a %T", old))
	}

	var allErrs field.ErrorList
	detailMsg := "updates on name, vhost, and rabbitmqClusterReference are all forbidden"
	if e.Spec.Name != oldExchange.Spec.Name {
		return apierrors.NewForbidden(e.GroupResource(), e.Name,
			field.Forbidden(field.NewPath("spec", "name"), detailMsg))
	}

	if e.Spec.Vhost != oldExchange.Spec.Vhost {
		return apierrors.NewForbidden(e.GroupResource(), e.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if !oldExchange.Spec.RabbitmqClusterReference.Matches(&e.Spec.RabbitmqClusterReference) {
		return apierrors.NewForbidden(e.GroupResource(), e.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}

	if e.Spec.Type != oldExchange.Spec.Type {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "type"),
			e.Spec.Type,
			"exchange type cannot be updated",
		))
	}

	if e.Spec.AutoDelete != oldExchange.Spec.AutoDelete {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "autoDelete"),
			e.Spec.AutoDelete,
			"autoDelete cannot be updated",
		))
	}

	if e.Spec.Durable != oldExchange.Spec.Durable {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "durable"),
			e.Spec.AutoDelete,
			"durable cannot be updated",
		))
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(GroupVersion.WithKind("Exchange").GroupKind(), e.Name, allErrs)
}

// no validation on delete
func (e *Exchange) ValidateDelete() error {
	return nil
}
