package v1beta1

import (
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (q *Queue) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(q).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-rabbitmq-com-v1beta1-queue,mutating=false,failurePolicy=fail,groups=rabbitmq.com,resources=queues,versions=v1beta1,name=vqueue.kb.io,sideEffects=none,admissionReviewVersions=v1sideEffects=none,admissionReviewVersions=v1

var _ webhook.Validator = &Queue{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
// either rabbitmqClusterReference.name or rabbitmqClusterReference.connectionSecret must be provided but not both
func (q *Queue) ValidateCreate() error {
	if q.Spec.Type == "quorum" && q.Spec.Durable == false {
		return apierrors.NewForbidden(q.GroupResource(), q.Name,
			field.Forbidden(field.NewPath("spec", "durable"),
				"Quorum queues must have durable set to true"))
	}
	return q.Spec.RabbitmqClusterReference.ValidateOnCreate(q.GroupResource(), q.Name)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
// returns error type 'forbidden' for updates that the controller chooses to disallow: queue name/vhost/rabbitmqClusterReference
// returns error type 'invalid' for updates that will be rejected by rabbitmq server: queue types/autoDelete/durable
// queue arguments not handled because implementation couldn't change
func (q *Queue) ValidateUpdate(old runtime.Object) error {
	oldQueue, ok := old.(*Queue)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a queue but got a %T", old))
	}

	var allErrs field.ErrorList
	detailMsg := "updates on name, vhost, and rabbitmqClusterReference are all forbidden"
	if q.Spec.Name != oldQueue.Spec.Name {
		return apierrors.NewForbidden(q.GroupResource(), q.Name,
			field.Forbidden(field.NewPath("spec", "name"), detailMsg))
	}

	if q.Spec.Vhost != oldQueue.Spec.Vhost {
		return apierrors.NewForbidden(q.GroupResource(), q.Name,
			field.Forbidden(field.NewPath("spec", "vhost"), detailMsg))
	}

	if !oldQueue.Spec.RabbitmqClusterReference.Matches(&q.Spec.RabbitmqClusterReference) {
		return apierrors.NewForbidden(q.GroupResource(), q.Name,
			field.Forbidden(field.NewPath("spec", "rabbitmqClusterReference"), detailMsg))
	}

	if q.Spec.Type != oldQueue.Spec.Type {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "type"),
			q.Spec.Type,
			"queue type cannot be updated",
		))
	}

	if q.Spec.AutoDelete != oldQueue.Spec.AutoDelete {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "autoDelete"),
			q.Spec.AutoDelete,
			"autoDelete cannot be updated",
		))
	}

	if q.Spec.Durable != oldQueue.Spec.Durable {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec", "durable"),
			q.Spec.AutoDelete,
			"durable cannot be updated",
		))
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(GroupVersion.WithKind("Queue").GroupKind(), q.Name, allErrs)
}

// no validation on delete
func (q *Queue) ValidateDelete() error {
	return nil
}
