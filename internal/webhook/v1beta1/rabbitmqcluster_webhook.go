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
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	rabbitmqcomv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
)

var rabbitmqclusterlog = logf.Log.WithName("rabbitmqcluster-resource")

// SetupRabbitmqClusterWebhookWithManager registers the webhook for RabbitmqCluster in the manager.
func SetupRabbitmqClusterWebhookWithManager(mgr ctrl.Manager, defaulter RabbitmqClusterCustomDefaulter) error {
	return ctrl.NewWebhookManagedBy(mgr, &rabbitmqcomv1beta1.RabbitmqCluster{}).
		WithDefaulter(&defaulter).
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
