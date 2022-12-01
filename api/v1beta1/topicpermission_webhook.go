package v1beta1

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var topicpermissionlog = logf.Log.WithName("topicpermission-resource")

func (r *TopicPermission) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-rabbitmq-com-v1beta1-topicpermission,mutating=false,failurePolicy=fail,sideEffects=None,groups=rabbitmq.com,resources=topicpermissions,verbs=create;update,versions=v1beta1,name=vtopicpermission.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &TopicPermission{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (p *TopicPermission) ValidateCreate() error {
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

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (p *TopicPermission) ValidateUpdate(old runtime.Object) error {
	oldPermission, ok := old.(*TopicPermission)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a permission but got a %T", old))
	}

	var errorList field.ErrorList
	if p.Spec.User == "" && p.Spec.UserReference == nil {
		errorList = append(errorList, field.Required(field.NewPath("spec", "user and userReference"),
			"must specify either spec.user or spec.userReference"))
		return apierrors.NewInvalid(GroupVersion.WithKind("TopicPermission").GroupKind(), p.Name, errorList)
	}

	if p.Spec.User != "" && p.Spec.UserReference != nil {
		errorList = append(errorList, field.Required(field.NewPath("spec", "user and userReference"),
			"cannot specify spec.user and spec.userReference at the same time"))
		return apierrors.NewInvalid(GroupVersion.WithKind("TopicPermission").GroupKind(), p.Name, errorList)
	}

	detailMsg := "updates on exchange, user, userReference, vhost and rabbitmqClusterReference are all forbidden"
	if p.Spec.Permissions.Exchange != oldPermission.Spec.Permissions.Exchange {
		return apierrors.NewForbidden(p.GroupResource(), p.Name,
			field.Forbidden(field.NewPath("spec", "permissions", "exchange"), detailMsg))
	}

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

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *TopicPermission) ValidateDelete() error {
	return nil
}
