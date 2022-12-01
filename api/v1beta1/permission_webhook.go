package v1beta1

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (p *Permission) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(p).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-permission,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=permissions,versions=v1beta1,name=vpermission.kb.io,sideEffects=none,admissionReviewVersions=v1

var _ webhook.Validator = &Permission{}

// ValidateCreate checks if only one of spec.user and spec.userReference is specified
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (p *Permission) ValidateCreate() error {
	var errorList field.ErrorList
	if p.Spec.User == "" && p.Spec.UserReference == nil {
		errorList = append(errorList, field.Required(field.NewPath("spec", "user and userReference"),
			"must specify either spec.user or spec.userReference"))
		return apierrors.NewInvalid(GroupVersion.WithKind("Permission").GroupKind(), p.Name, errorList)
	}

	if p.Spec.User != "" && p.Spec.UserReference != nil {
		errorList = append(errorList, field.Required(field.NewPath("spec", "user and userReference"),
			"cannot specify spec.user and spec.userReference at the same time"))
		return apierrors.NewInvalid(GroupVersion.WithKind("Permission").GroupKind(), p.Name, errorList)
	}

	return p.Spec.RabbitmqClusterReference.ValidateOnCreate(p.GroupResource(), p.Name)
}

// ValidateUpdate do not allow updates on spec.vhost, spec.user, spec.userReference, and spec.rabbitmqClusterReference
// updates on spec.permissions are allowed
// only one of spec.user and spec.userReference can be specified
func (p *Permission) ValidateUpdate(old runtime.Object) error {
	oldPermission, ok := old.(*Permission)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a permission but got a %T", old))
	}

	var errorList field.ErrorList
	if p.Spec.User == "" && p.Spec.UserReference == nil {
		errorList = append(errorList, field.Required(field.NewPath("spec", "user and userReference"),
			"must specify either spec.user or spec.userReference"))
		return apierrors.NewInvalid(GroupVersion.WithKind("Permission").GroupKind(), p.Name, errorList)
	}

	if p.Spec.User != "" && p.Spec.UserReference != nil {
		errorList = append(errorList, field.Required(field.NewPath("spec", "user and userReference"),
			"cannot specify spec.user and spec.userReference at the same time"))
		return apierrors.NewInvalid(GroupVersion.WithKind("Permission").GroupKind(), p.Name, errorList)
	}

	detailMsg := "updates on user, userReference, vhost and rabbitmqClusterReference are all forbidden"
	if p.Spec.User != oldPermission.Spec.User {
		return apierrors.NewForbidden(p.GroupResource(), p.Name,
			field.Forbidden(field.NewPath("spec", "user"), detailMsg))
	}

	if userReferenceUpdated(p.Spec.UserReference, oldPermission.Spec.UserReference) {
		return apierrors.NewForbidden(p.GroupResource(), p.Name,
			field.Forbidden(field.NewPath("spec", "userReference"), detailMsg))
	}

	if p.Spec.Vhost != oldPermission.Spec.Vhost {
		return apierrors.NewForbidden(p.GroupResource(), p.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if !oldPermission.Spec.RabbitmqClusterReference.Matches(&p.Spec.RabbitmqClusterReference) {
		return apierrors.NewForbidden(p.GroupResource(), p.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}
	return nil
}

// returns true if userReference, which is a pointer to corev1.LocalObjectReference, has changed
func userReferenceUpdated(new, old *corev1.LocalObjectReference) bool {
	if new == nil && old == nil {
		return false
	}
	if (new == nil && old != nil) ||
		(new != nil && old == nil) {
		return true
	}
	if new.Name != old.Name {
		return true
	}
	return false
}

// ValidateDelete no validation on delete
func (p *Permission) ValidateDelete() error {
	return nil
}
