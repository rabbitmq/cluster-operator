/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package topologycontrollers

import (
	"context"
	"errors"
	"fmt"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"

	"github.com/rabbitmq/cluster-operator/internal/rabbitmqclient"
	"github.com/rabbitmq/cluster-operator/internal/topology"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientretry "k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var apiGVStr = rabbitmqv1beta1.GroupVersion.String()

const (
	ownerKey  = ".metadata.controller"
	ownerKind = "User"
)

// +kubebuilder:rbac:groups=rabbitmq.com,resources=users,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=users/finalizers,verbs=update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=users/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create

type UserReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *UserReconciler) declareCredentials(ctx context.Context, user *rabbitmqv1beta1.User) (string, error) {
	logger := ctrl.LoggerFrom(ctx)

	username, password, err := r.generateCredentials(ctx, user)
	if err != nil {
		logger.Error(err, "failed to generate credentials")
		return "", err
	}
	// Password wasn't in the provided input secret we need to generate a random one
	if password == "" {
		password, err = internal.RandomEncodedString(24)
		if err != nil {
			return "", fmt.Errorf("failed to generate random password: %w", err)
		}

	}

	logger.Info("Credentials generated for User", "user", user.Name, "generatedUsername", username)

	credentialSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      user.ObjectMeta.Name + "-user-credentials",
			Namespace: user.ObjectMeta.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		// The format of the generated Secret conforms to the Provisioned Service
		// type Spec. For more information, see https://k8s-service-bindings.github.io/spec/#provisioned-service.
		Data: map[string][]byte{
			"username": []byte(username),
			"password": []byte(password),
		},
	}

	var operationResult controllerutil.OperationResult
	err = clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		var apiError error
		operationResult, apiError = controllerutil.CreateOrUpdate(ctx, r.Client, &credentialSecret, func() error {
			if err := controllerutil.SetControllerReference(user, &credentialSecret, r.Scheme); err != nil {
				return fmt.Errorf("failed setting controller reference: %v", err)
			}
			// required for OpenShift compatibility. See:
			// https://github.com/rabbitmq/cluster-operator/blob/057b61eb50102a66f504b31464e5956526cbdc90/internal/resource/statefulset.go#L220-L226
			// https://github.com/rabbitmq/messaging-topology-operator/issues/194
			for i := range credentialSecret.ObjectMeta.OwnerReferences {
				credentialSecret.ObjectMeta.OwnerReferences[i].BlockOwnerDeletion = pointer.BoolPtr(false)
			}
			return nil
		})
		return apiError
	})
	if err != nil {
		msg := fmt.Sprintf("failed to create/update credentials secret: %s", string(operationResult))
		logger.Error(err, msg)
		return "", err
	}

	logger.Info("Successfully declared credentials secret", "secret", credentialSecret.Name, "namespace", credentialSecret.Namespace)
	return username, nil
}

func (r *UserReconciler) generateCredentials(ctx context.Context, user *rabbitmqv1beta1.User) (string, string, error) {
	logger := ctrl.LoggerFrom(ctx)

	var err error
	msg := fmt.Sprintf("generating/importing credentials for User %s: %#v", user.Name, user)
	logger.Info(msg)

	if user.Spec.ImportCredentialsSecret != nil {
		logger.Info("An import secret was provided in the user spec", "user", user.Name, "secretName", user.Spec.ImportCredentialsSecret.Name)
		return r.importCredentials(ctx, user.Spec.ImportCredentialsSecret.Name, user.Namespace)
	}

	username, err := internal.RandomEncodedString(24)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate random username: %w", err)
	}
	password, err := internal.RandomEncodedString(24)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate random password: %w", err)
	}
	return username, password, nil
}

func (r *UserReconciler) importCredentials(ctx context.Context, secretName, secretNamespace string) (string, string, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Importing user credentials from provided Secret", "secretName", secretName, "secretNamespace", secretNamespace)

	var credentialsSecret corev1.Secret
	err := r.Client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: secretNamespace}, &credentialsSecret)
	if err != nil {
		return "", "", fmt.Errorf("could not find password secret %s in namespace %s; Err: %w", secretName, secretNamespace, err)
	}
	username, ok := credentialsSecret.Data["username"]
	if !ok {
		return "", "", fmt.Errorf("could not find username key in credentials secret: %s", credentialsSecret.Name)
	}
	password, ok := credentialsSecret.Data["password"]
	if !ok {
		return string(username), "", nil
	}

	logger.Info("Retrieved credentials from Secret", "secretName", secretName, "retrievedUsername", string(username))
	return string(username), string(password), nil
}

func (r *UserReconciler) setUserStatus(ctx context.Context, user *rabbitmqv1beta1.User, username string) error {
	logger := ctrl.LoggerFrom(ctx)

	credentials := &corev1.LocalObjectReference{
		Name: user.Name + "-user-credentials",
	}
	user.Status.Credentials = credentials
	user.Status.Username = username
	if err := r.Status().Update(ctx, user); err != nil {
		msg := "Failed to update secret status credentials"
		logger.Error(err, msg, "user", user.Name, "secretRef", credentials)
		return err
	}
	logger.Info("Successfully updated secret status credentials", "user", user.Name, "secretRef", credentials)
	return nil
}

func (r *UserReconciler) DeclareFunc(ctx context.Context, client rabbitmqclient.Client, obj rabbitmqv1beta1.TopologyResource) error {
	logger := ctrl.LoggerFrom(ctx)
	user := obj.(*rabbitmqv1beta1.User)
	if user.Status.Credentials == nil || user.Status.Username == "" {
		var username string
		if user.Status.Credentials != nil && user.Status.Username == "" {
			// Only run once for migration to set user.Status.Username on existing resources
			credentials, err := r.getUserCredentials(ctx, user)
			if err != nil {
				return err
			}
			username = string(credentials.Data["username"])
		} else {
			logger.Info("User does not yet have a Credentials Secret; generating", "user", user.Name)
			var err error
			if username, err = r.declareCredentials(ctx, user); err != nil {
				return err
			}
		}
		if err := r.setUserStatus(ctx, user, username); err != nil {
			return err
		}
	}

	credentials, err := r.getUserCredentials(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to retrieve user credentials secret from status; user.status: %v", user.Status)
	}
	logger.Info("Retrieved credentials for user", "user", user.Name, "credentials", credentials.Name)

	userSettings, err := internal.GenerateUserSettings(credentials, user.Spec.Tags)
	if err != nil {
		return fmt.Errorf("failed to generate user settings from credential: %w", err)
	}
	logger.Info("Generated user settings", "user", user.Name, "settings", userSettings)

	return validateResponse(client.PutUser(userSettings.Name, userSettings))
}

func (r *UserReconciler) getUserCredentials(ctx context.Context, user *rabbitmqv1beta1.User) (*corev1.Secret, error) {
	if user.Status.Credentials == nil {
		return nil, fmt.Errorf("this User does not yet have a Credentials Secret created")
	}
	credentials := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: user.Status.Credentials.Name, Namespace: user.Namespace}, credentials); err != nil {
		return nil, err
	}
	return credentials, nil
}

func (r *UserReconciler) deleteUserFromSecret(ctx context.Context, client rabbitmqclient.Client, user *rabbitmqv1beta1.User) error {
	logger := ctrl.LoggerFrom(ctx)

	credentials, err := r.getUserCredentials(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to retrieve user credentials secret from status;"+
			" user.status: %v, err: %w", user.Status, err)
	}

	err = validateResponseForDeletion(client.DeleteUser(string(credentials.Data["username"])))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find user in rabbitmq server; already deleted", "user", user.Name)
	} else if err != nil {
		return err
	}
	return nil
}

func (r *UserReconciler) DeleteFunc(ctx context.Context, client rabbitmqclient.Client, obj rabbitmqv1beta1.TopologyResource) error {
	logger := ctrl.LoggerFrom(ctx)
	user := obj.(*rabbitmqv1beta1.User)

	err := validateResponseForDeletion(client.DeleteUser(user.Status.Username))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find user in rabbitmq server; already deleted", "user", user.Name)
	} else if err != nil {
		return err
	}
	return nil
}
