package controllers

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

func (r *RabbitmqClusterReconciler) reconcileTLS(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) error {
	if rabbitmqCluster.DisableNonTLSListeners() && !rabbitmqCluster.TLSEnabled() {
		err := errors.NewBadRequest("TLS must be enabled if disableNonTLSListeners is set to true")
		r.Recorder.Event(rabbitmqCluster, corev1.EventTypeWarning, "TLSError", err.Error())
		ctrl.LoggerFrom(ctx).Error(err, "Error setting up TLS")
		return err
	}

	if rabbitmqCluster.TLSEnabled() {
		if err := r.checkTLSSecrets(ctx, rabbitmqCluster); err != nil {
			return err
		}
	}
	return nil
}

func (r *RabbitmqClusterReconciler) checkTLSSecrets(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) error {
	logger := ctrl.LoggerFrom(ctx)
	secretName := rabbitmqCluster.Spec.TLS.SecretName
	logger.Info("TLS enabled, looking for secret", "secret", secretName)

	// check if secret exists
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: rabbitmqCluster.Namespace, Name: secretName}, secret); err != nil {
		r.Recorder.Event(rabbitmqCluster, corev1.EventTypeWarning, "TLSError",
			fmt.Sprintf("Failed to get TLS secret %s in namespace %s: %v", secretName, rabbitmqCluster.Namespace, err.Error()))
		logger.Error(err, "Error setting up TLS", "namespace", rabbitmqCluster.Namespace, "name", rabbitmqCluster.Name)
		return err
	}
	// check if secret has the right keys
	_, hasTLSKey := secret.Data["tls.key"]
	_, hasTLSCert := secret.Data["tls.crt"]
	if !hasTLSCert || !hasTLSKey {
		err := errors.NewBadRequest(fmt.Sprintf("TLS secret %s in namespace %s does not have the fields tls.crt and tls.key", secretName, rabbitmqCluster.Namespace))
		r.Recorder.Event(rabbitmqCluster, corev1.EventTypeWarning, "TLSError", err.Error())
		logger.Error(err, "Error setting up TLS")
		return err
	}

	// Mutual TLS: check if CA certificate is stored in a separate secret
	if rabbitmqCluster.MutualTLSEnabled() {
		if !rabbitmqCluster.SingleTLSSecret() {
			secretName := rabbitmqCluster.Spec.TLS.CaSecretName
			logger.Info("mutual TLS enabled, looking for CA certificate secret", "secret", secretName)

			// check if secret exists
			secret = &corev1.Secret{}
			if err := r.Get(ctx, types.NamespacedName{Namespace: rabbitmqCluster.Namespace, Name: secretName}, secret); err != nil {
				r.Recorder.Event(rabbitmqCluster, corev1.EventTypeWarning, "TLSError",
					fmt.Sprintf("Failed to get CA certificate secret %v in namespace %v: %v", secretName, rabbitmqCluster.Namespace, err.Error()))
				logger.Error(err, "Error setting up TLS")
				return err
			}
		}

		// Mutual TLS: verify that CA certificate is present in secret
		if _, hasCaCert := secret.Data["ca.crt"]; !hasCaCert {
			err := errors.NewBadRequest(fmt.Sprintf("TLS secret %s in namespace %s does not have the field ca.crt", rabbitmqCluster.Spec.TLS.CaSecretName, rabbitmqCluster.Namespace))
			r.Recorder.Event(rabbitmqCluster, corev1.EventTypeWarning, "TLSError", err.Error())
			logger.Error(err, "Error setting up TLS")
			return err
		}
	}
	return nil
}
