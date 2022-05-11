package controllers

import (
	"context"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/resource"
	corev1 "k8s.io/api/core/v1"
	"reflect"
)

// reconcileStatus sets status.defaultUser (secret and service reference) and status.binding.
// when vault is used as secret backend for default user, no user secret object is created
// therefore only status.defaultUser.serviceReference is set.
// status.binding exposes the default user secret which contains the binding
// information for this RabbitmqCluster.
// Default user secret implements the service binding Provisioned Service
// See: https://k8s-service-bindings.github.io/spec/#provisioned-service
func (r *RabbitmqClusterReconciler) reconcileStatus(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) error {
	var binding *corev1.LocalObjectReference

	defaultUserStatus := &rabbitmqv1beta1.RabbitmqClusterDefaultUser{
		ServiceReference: &rabbitmqv1beta1.RabbitmqClusterServiceReference{
			Name:      rmq.ChildResourceName(""),
			Namespace: rmq.Namespace,
		},
	}

	if !rmq.VaultDefaultUserSecretEnabled() {
		defaultUserStatus.SecretReference = &rabbitmqv1beta1.RabbitmqClusterSecretReference{
			Name:      rmq.ChildResourceName(resource.DefaultUserSecretName),
			Namespace: rmq.Namespace,
			Keys: map[string]string{
				"username": "username",
				"password": "password",
			},
		}
		binding = &corev1.LocalObjectReference{
			Name: rmq.ChildResourceName(resource.DefaultUserSecretName),
		}
	}

	if !reflect.DeepEqual(rmq.Status.DefaultUser, defaultUserStatus) || !reflect.DeepEqual(rmq.Status.Binding, binding) {
		rmq.Status.DefaultUser = defaultUserStatus
		rmq.Status.Binding = binding
		if err := r.Status().Update(ctx, rmq); err != nil {
			return err
		}
	}

	return nil
}
