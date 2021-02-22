package controllers

import (
	"context"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/resource"
	corev1 "k8s.io/api/core/v1"
	"reflect"
)

func (r *RabbitmqClusterReconciler) setDefaultUserStatus(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) error {

	defaultUserStatus := &rabbitmqv1beta1.RabbitmqClusterDefaultUser{}

	serviceRef := &rabbitmqv1beta1.RabbitmqClusterServiceReference{
		Name:      rmq.ChildResourceName(""),
		Namespace: rmq.Namespace,
	}
	defaultUserStatus.ServiceReference = serviceRef

	secretRef := &rabbitmqv1beta1.RabbitmqClusterSecretReference{
		Name:      rmq.ChildResourceName(resource.DefaultUserSecretName),
		Namespace: rmq.Namespace,
		Keys: map[string]string{
			"username": "username",
			"password": "password",
		},
	}
	defaultUserStatus.SecretReference = secretRef

	if !reflect.DeepEqual(rmq.Status.DefaultUser, defaultUserStatus) {
		rmq.Status.DefaultUser = defaultUserStatus
		if err := r.Status().Update(ctx, rmq); err != nil {
			return err
		}
	}

	return nil
}

// Status.Binding exposes the default user secret which contains the binding
// information for this RabbitmqCluster.
// Default user secret implements the service binding Provisioned Service
// See: https://k8s-service-bindings.github.io/spec/#provisioned-service
func (r *RabbitmqClusterReconciler) setBinding(ctx context.Context, rmq *rabbitmqv1beta1.RabbitmqCluster) error {
	binding := &corev1.LocalObjectReference{
		Name: rmq.ChildResourceName(resource.DefaultUserSecretName),
	}
	if !reflect.DeepEqual(rmq.Status.Binding, binding) {
		rmq.Status.Binding = binding
		if err := r.Status().Update(ctx, rmq); err != nil {
			return err
		}
	}
	return nil
}
