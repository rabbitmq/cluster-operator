package v1beta1

import (
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (s *Shovel) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(s).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-shovel,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=shovels,versions=v1beta1,name=vshovel.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.Validator = &Shovel{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (s *Shovel) ValidateCreate() error {
	return s.Spec.RabbitmqClusterReference.ValidateOnCreate(s.GroupResource(), s.Name)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (s *Shovel) ValidateUpdate(old runtime.Object) error {
	oldShovel, ok := old.(*Shovel)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a shovel but got a %T", oldShovel))
	}

	detailMsg := "updates on name, vhost and rabbitmqClusterReference are all forbidden"
	if s.Spec.Name != oldShovel.Spec.Name {
		return apierrors.NewForbidden(s.GroupResource(), s.Name,
			field.Forbidden(field.NewPath("spec", "name"), detailMsg))
	}

	if s.Spec.Vhost != oldShovel.Spec.Vhost {
		return apierrors.NewForbidden(s.GroupResource(), s.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if !oldShovel.Spec.RabbitmqClusterReference.Matches(&s.Spec.RabbitmqClusterReference) {
		return apierrors.NewForbidden(s.GroupResource(), s.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}
	return nil
}

// no validation on delete
func (s *Shovel) ValidateDelete() error {
	return nil
}
