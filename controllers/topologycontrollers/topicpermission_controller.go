package topologycontrollers

import (
	"context"
	"errors"
	"fmt"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/rabbitmqclient"
	"github.com/rabbitmq/cluster-operator/internal/topology"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

//+kubebuilder:rbac:groups=rabbitmq.com,resources=topicpermissions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rabbitmq.com,resources=topicpermissions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=rabbitmq.com,resources=topicpermissions/finalizers,verbs=update

type TopicPermissionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *TopicPermissionReconciler) DeclareFunc(ctx context.Context, client rabbitmqclient.Client, obj rabbitmqv1beta1.TopologyResource) error {
	permission := obj.(*rabbitmqv1beta1.TopicPermission)
	user := &rabbitmqv1beta1.User{}
	username := permission.Spec.User
	if permission.Spec.UserReference != nil {
		var err error
		if user, err = getUsernameFromUser(ctx, r.Client, permission.Namespace, permission.Spec.UserReference.Name); err != nil {
			return err
		} else if user != nil {
			username = user.Status.Username
		}
	}
	if username == "" {
		return fmt.Errorf("failed create Permission, missing User")
	}
	if user.Name != "" {
		if err := controllerutil.SetControllerReference(user, permission, r.Scheme); err != nil {
			return fmt.Errorf("failed set controller reference: %v", err)
		}
		if err := r.Client.Update(ctx, permission); err != nil {
			return fmt.Errorf("failed to Update object with controller reference: %w", err)
		}
	}
	return validateResponse(client.UpdateTopicPermissionsIn(permission.Spec.Vhost, username, internal.GenerateTopicPermissions(permission)))
}

func (r *TopicPermissionReconciler) DeleteFunc(ctx context.Context, client rabbitmqclient.Client, obj rabbitmqv1beta1.TopologyResource) error {
	logger := ctrl.LoggerFrom(ctx)
	permission := obj.(*rabbitmqv1beta1.TopicPermission)
	username := permission.Spec.User
	if permission.Spec.UserReference != nil {
		if user, err := getUsernameFromUser(ctx, r.Client, permission.Namespace, permission.Spec.UserReference.Name); err != nil {
			return err
		} else if user != nil {
			username = user.Status.Username
		}
	}
	if username == "" {
		logger.Info("user already removed; no need to delete topic permission")
	} else if err := r.clearTopicPermission(ctx, client, permission, username); err != nil {
		return err
	}
	return removeFinalizer(ctx, r.Client, permission)
}

func (r *TopicPermissionReconciler) clearTopicPermission(ctx context.Context, client rabbitmqclient.Client, permission *rabbitmqv1beta1.TopicPermission, user string) error {
	logger := ctrl.LoggerFrom(ctx)
	err := validateResponseForDeletion(client.DeleteTopicPermissionsIn(permission.Spec.Vhost, user, permission.Spec.Permissions.Exchange))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find user or vhost in rabbitmq server; no need to delete permission", "user", user, "vhost", permission.Spec.Vhost)
		return nil
	}
	return err
}
