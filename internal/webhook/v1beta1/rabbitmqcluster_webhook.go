/*
RabbitMQ Cluster Operator

Copyright 2020-2022 VMware, Inc. All Rights Reserved.
Copyright 2022-2026 Broadcom. All Rights Reserved.

This product is licensed to you under the Mozilla Public license,
Version 2.0 (the "License").  You may not use this product except in
compliance with the Mozilla Public License.

This product may include a number of subcomponents with separate
copyright notices and license terms. Your use of these subcomponents
is subject to the terms and conditions of the subcomponent's license,
as noted in the LICENSE file.
*/

package v1beta1

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	rabbitmqcomv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
)

var rabbitmqclusterlog = logf.Log.WithName("rabbitmqcluster-webhook")

// SetupRabbitmqClusterWebhookWithManager registers the webhook for RabbitmqCluster in the manager.
func SetupRabbitmqClusterWebhookWithManager(mgr ctrl.Manager, defaulter RabbitmqClusterCustomDefaulter) error {
	return ctrl.NewWebhookManagedBy(mgr, &rabbitmqcomv1beta1.RabbitmqCluster{}).
		WithDefaulter(&defaulter).
		WithValidator(&RabbitmqClusterCustomValidator{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-rabbitmq-com-v1beta1-rabbitmqcluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=rabbitmq.com,resources=rabbitmqclusters,verbs=create;update,versions=v1beta1,name=mrabbitmqcluster-v1beta1.kb.io,admissionReviewVersions=v1

// RabbitmqClusterCustomDefaulter sets default values on RabbitmqCluster resources at admission time.
// Defaults are applied only when the relevant field is unset, preserving any value explicitly configured by the user.
//
// +kubebuilder:object:generate=false
type RabbitmqClusterCustomDefaulter struct {
	DefaultRabbitmqImage    string
	DefaultImagePullSecrets string
	DefaultUserUpdaterImage string
}

// +kubebuilder:webhook:path=/validate-rabbitmq-com-v1beta1-rabbitmqcluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=rabbitmq.com,resources=rabbitmqclusters,verbs=create;update,versions=v1beta1,name=vrabbitmqcluster-v1beta1.kb.io,admissionReviewVersions=v1

// RabbitmqClusterCustomValidator rejects RabbitmqCluster resources that request
// host-level or privileged access via spec.override.statefulSet.spec.template.spec.
//
// +kubebuilder:object:generate=false
type RabbitmqClusterCustomValidator struct{}

// ValidateCreate implements admission.Validator.
func (v *RabbitmqClusterCustomValidator) ValidateCreate(_ context.Context, obj *rabbitmqcomv1beta1.RabbitmqCluster) (admission.Warnings, error) {
	return nil, validatePodSpecOverride(obj)
}

// ValidateUpdate implements admission.Validator.
func (v *RabbitmqClusterCustomValidator) ValidateUpdate(_ context.Context, _, newObj *rabbitmqcomv1beta1.RabbitmqCluster) (admission.Warnings, error) {
	return nil, validatePodSpecOverride(newObj)
}

// ValidateDelete implements admission.Validator.
func (v *RabbitmqClusterCustomValidator) ValidateDelete(_ context.Context, _ *rabbitmqcomv1beta1.RabbitmqCluster) (admission.Warnings, error) {
	return nil, nil
}

func validatePodSpecOverride(cluster *rabbitmqcomv1beta1.RabbitmqCluster) error {
	override := cluster.Spec.Override.StatefulSet
	if override == nil || override.Spec == nil || override.Spec.Template == nil || override.Spec.Template.Spec == nil {
		return nil
	}

	podSpec := override.Spec.Template.Spec
	specPath := field.NewPath("spec", "override", "statefulSet", "spec", "template", "spec")
	var allErrs field.ErrorList

	if podSpec.HostPID {
		allErrs = append(allErrs, field.Forbidden(specPath.Child("hostPID"), "hostPID is not permitted in override"))
	}
	if podSpec.HostNetwork {
		allErrs = append(allErrs, field.Forbidden(specPath.Child("hostNetwork"), "hostNetwork is not permitted in override"))
	}
	if podSpec.HostIPC {
		allErrs = append(allErrs, field.Forbidden(specPath.Child("hostIPC"), "hostIPC is not permitted in override"))
	}
	if podSpec.ServiceAccountName != "" {
		allErrs = append(allErrs, field.Forbidden(specPath.Child("serviceAccountName"), "serviceAccountName is managed by the operator and cannot be overridden"))
	}
	for i, vol := range podSpec.Volumes {
		if vol.HostPath != nil {
			allErrs = append(allErrs, field.Forbidden(specPath.Child("volumes").Index(i).Child("hostPath"),
				"hostPath volumes are not permitted in override"))
		}
	}
	for i, c := range podSpec.Containers {
		cPath := specPath.Child("containers").Index(i)
		if c.SecurityContext != nil && c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged {
			allErrs = append(allErrs, field.Forbidden(cPath.Child("securityContext", "privileged"), "privileged containers are not permitted in override"))
		}
		if c.SecurityContext != nil && c.SecurityContext.AllowPrivilegeEscalation != nil && *c.SecurityContext.AllowPrivilegeEscalation {
			allErrs = append(allErrs, field.Forbidden(cPath.Child("securityContext", "allowPrivilegeEscalation"), "allowPrivilegeEscalation is not permitted in override"))
		}
	}
	for i, c := range podSpec.InitContainers {
		cPath := specPath.Child("initContainers").Index(i)
		if c.SecurityContext != nil && c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged {
			allErrs = append(allErrs, field.Forbidden(cPath.Child("securityContext", "privileged"), "privileged containers are not permitted in override"))
		}
		if c.SecurityContext != nil && c.SecurityContext.AllowPrivilegeEscalation != nil && *c.SecurityContext.AllowPrivilegeEscalation {
			allErrs = append(allErrs, field.Forbidden(cPath.Child("securityContext", "allowPrivilegeEscalation"), "allowPrivilegeEscalation is not permitted in override"))
		}
	}

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: "rabbitmq.com", Kind: "RabbitmqCluster"},
			cluster.Name,
			allErrs,
		)
	}
	return nil
}

// Default implements webhook.CustomDefaulter.
func (d *RabbitmqClusterCustomDefaulter) Default(_ context.Context, obj *rabbitmqcomv1beta1.RabbitmqCluster) error {
	rabbitmqclusterlog.Info("Defaulting for RabbitmqCluster", "name", obj.GetName())

	if obj.Spec.Image == "" {
		obj.Spec.Image = d.DefaultRabbitmqImage
	}

	if obj.Spec.ImagePullSecrets == nil && d.DefaultImagePullSecrets != "" {
		for ref := range strings.SplitSeq(d.DefaultImagePullSecrets, ",") {
			if ref != "" {
				obj.Spec.ImagePullSecrets = append(obj.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: ref})
			}
		}
	}

	if obj.VaultEnabled() && obj.Spec.SecretBackend.Vault.DefaultUserUpdaterImage == nil {
		image := d.DefaultUserUpdaterImage
		obj.Spec.SecretBackend.Vault.DefaultUserUpdaterImage = &image
	}

	return nil
}
