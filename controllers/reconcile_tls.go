package controllers

import (
	"context"
	"fmt"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *RabbitmqClusterReconciler) checkTLSSecrets(ctx context.Context, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) (ctrl.Result, error) {
	logger := r.Log
	secretName := rabbitmqCluster.Spec.TLS.SecretName
	logger.Info("TLS set, looking for secret", "secret", secretName, "namespace", rabbitmqCluster.Namespace)

	// check if secret exists
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: rabbitmqCluster.Namespace, Name: secretName}, secret); err != nil {
		r.Recorder.Event(rabbitmqCluster, corev1.EventTypeWarning, "TLSError",
			fmt.Sprintf("Failed to get TLS secret %v in namespace %v: %v", secretName, rabbitmqCluster.Namespace, err.Error()))
		return ctrl.Result{}, err
	}
	// check if secret has the right keys
	_, hasTLSKey := secret.Data["tls.key"]
	_, hasTLSCert := secret.Data["tls.crt"]
	if !hasTLSCert || !hasTLSKey {
		r.Recorder.Event(rabbitmqCluster, corev1.EventTypeWarning, "TLSError",
			fmt.Sprintf("The TLS secret %v in namespace %v must have the fields tls.crt and tls.key", secretName, rabbitmqCluster.Namespace))

		return ctrl.Result{}, errors.NewBadRequest("The TLS secret must have the fields tls.crt and tls.key")
	}

	// Mutual TLS: check if CA certificate is stored in a separate secret
	if rabbitmqCluster.MutualTLSEnabled() {
		if !rabbitmqCluster.SingleTLSSecret() {
			secretName := rabbitmqCluster.Spec.TLS.CaSecretName
			logger.Info("mutual TLS set, looking for CA certificate secret", "secret", secretName, "namespace", rabbitmqCluster.Namespace)

			// check if secret exists
			secret = &corev1.Secret{}
			if err := r.Get(ctx, types.NamespacedName{Namespace: rabbitmqCluster.Namespace, Name: secretName}, secret); err != nil {
				r.Recorder.Event(rabbitmqCluster, corev1.EventTypeWarning, "TLSError",
					fmt.Sprintf("Failed to get CA certificate secret %v in namespace %v: %v", secretName, rabbitmqCluster.Namespace, err.Error()))
				return ctrl.Result{}, err
			}
		}
		// Mutual TLS: verify that CA certificate is present in secret
		_, hasCaCert := secret.Data["ca.crt"]
		if !hasCaCert {
			r.Recorder.Event(rabbitmqCluster, corev1.EventTypeWarning, "TLSError",
				fmt.Sprintf("The TLS secret %v in namespace %v must have the field ca.crt", rabbitmqCluster.Spec.TLS.CaSecretName, rabbitmqCluster.Namespace))

			return ctrl.Result{}, errors.NewBadRequest("The TLS secret must have the field ca.crt")
		}
	}
	return ctrl.Result{}, nil
}
