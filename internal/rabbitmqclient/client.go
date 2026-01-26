/*
RabbitMQ Cluster Operator

Copyright 2020 VMware, Inc. All Rights Reserved.

This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package rabbitmqclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClientInfo contains the information needed to make HTTP requests to RabbitMQ management API
type ClientInfo struct {
	BaseURL   string
	Username  string
	Password  string
	Transport *http.Transport
}

// GetRabbitmqClientForPod creates a rabbithole client for a specific pod using its stable DNS name.
// It fetches credentials from the default user secret and connects directly to the pod via its stable DNS.
func GetRabbitmqClientForPod(ctx context.Context, k8sClient client.Reader, rmq *rabbitmqv1beta1.RabbitmqCluster, podName string) (*rabbithole.Client, error) {
	info, err := GetClientInfoForPod(ctx, k8sClient, rmq, podName)
	if err != nil {
		return nil, err
	}

	var rabbitmqClient *rabbithole.Client
	if rmq.Spec.TLS.DisableNonTLSListeners {
		rabbitmqClient, err = rabbithole.NewTLSClient(info.BaseURL, info.Username, info.Password, info.Transport)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS rabbithole client for pod: %w", err)
		}
	} else {
		rabbitmqClient, err = rabbithole.NewClient(info.BaseURL, info.Username, info.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to create rabbithole client for pod: %w", err)
		}
	}

	return rabbitmqClient, nil
}

// GetClientInfoForPod creates ClientInfo for a specific pod using its stable DNS name.
// This is useful for checking individual pods instead of going through the service.
func GetClientInfoForPod(ctx context.Context, k8sClient client.Reader, rmq *rabbitmqv1beta1.RabbitmqCluster, podName string) (*ClientInfo, error) {
	// Fetch the default user secret
	secretName := rmq.ChildResourceName("default-user")
	secret := &corev1.Secret{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      secretName,
		Namespace: rmq.Namespace,
	}, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get default user secret: %w", err)
	}

	// Extract credentials
	username, ok := secret.Data["username"]
	if !ok {
		return nil, fmt.Errorf("username not found in secret %s", secretName)
	}

	password, ok := secret.Data["password"]
	if !ok {
		return nil, fmt.Errorf("password not found in secret %s", secretName)
	}

	// Build management API URL using pod stable DNS name
	var port int
	var scheme string

	// Create HTTP transport
	var transport *http.Transport

	// Construct the headless service name and pod FQDN
	headlessServiceName := rmq.ChildResourceName("nodes")
	podFQDN := fmt.Sprintf("%s.%s.%s.svc", podName, headlessServiceName, rmq.Namespace)

	// Use TLS only if there's no other alternative
	if rmq.Spec.TLS.DisableNonTLSListeners {
		port = 15671
		scheme = "https"

		// For TLS, we need to configure the transport
		tlsConfig := &tls.Config{}

		certPool, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("failed to get system cert pool: %w", err)
		}

		// If there's a CA certificate, add it to the cert pool
		if rmq.Spec.TLS.CaSecretName != "" {
			caSecret := &corev1.Secret{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      rmq.Spec.TLS.CaSecretName,
				Namespace: rmq.Namespace,
			}, caSecret)
			if err != nil {
				return nil, fmt.Errorf("failed to get CA secret: %w", err)
			}

			caCert, ok := caSecret.Data["ca.crt"]
			if !ok {
				return nil, fmt.Errorf("CA certificate not found in secret %s", rmq.Spec.TLS.CaSecretName)
			}
			if !certPool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("failed to append CA certificate to cert pool")
			}
			tlsConfig.RootCAs = certPool
		}

		// Use the pod's stable DNS name for TLS ServerName
		// This matches the SAN entries users typically configure in their certificates
		tlsConfig.ServerName = podFQDN

		transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	} else {
		port = 15672
		scheme = "http"
	}

	// Use pod stable DNS name
	baseURL := fmt.Sprintf("%s://%s:%d", scheme, podFQDN, port)

	return &ClientInfo{
		BaseURL:   baseURL,
		Username:  string(username),
		Password:  string(password),
		Transport: transport,
	}, nil
}
