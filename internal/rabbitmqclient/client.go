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

// GetRabbitmqClientForPod creates a rabbithole client for a specific pod using its IP address.
// It fetches credentials from the default user secret and connects directly to the pod IP.
func GetRabbitmqClientForPod(ctx context.Context, k8sClient client.Client, rmq *rabbitmqv1beta1.RabbitmqCluster, podIP string) (*rabbithole.Client, error) {
	info, err := GetClientInfoForPod(ctx, k8sClient, rmq, podIP)
	if err != nil {
		return nil, err
	}

	var rabbitmqClient *rabbithole.Client
	if rmq.Spec.TLS.SecretName != "" {
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

// GetClientInfoForPod creates ClientInfo for a specific pod IP.
// This is useful for checking individual pods instead of going through the service.
func GetClientInfoForPod(ctx context.Context, k8sClient client.Client, rmq *rabbitmqv1beta1.RabbitmqCluster, podIP string) (*ClientInfo, error) {
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

	// Build management API URL using pod IP
	var port int
	var scheme string

	// Create HTTP transport
	var transport *http.Transport

	// Check if TLS is enabled
	if rmq.Spec.TLS.SecretName != "" {
		port = 15671
		scheme = "https"

		// For TLS, we need to configure the transport
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true, // In production, you may want to verify certificates
		}

		// If there's a CA certificate, add it to the cert pool
		if rmq.Spec.TLS.CaSecretName != "" {
			caSecret := &corev1.Secret{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      rmq.Spec.TLS.CaSecretName,
				Namespace: rmq.Namespace,
			}, caSecret)
			if err == nil {
				if caCert, ok := caSecret.Data["ca.crt"]; ok {
					certPool := x509.NewCertPool()
					if certPool.AppendCertsFromPEM(caCert) {
						tlsConfig.RootCAs = certPool
						tlsConfig.InsecureSkipVerify = false
					}
				}
			}
		}

		transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	} else {
		port = 15672
		scheme = "http"
	}

	// Use pod IP directly instead of service name
	baseURL := fmt.Sprintf("%s://%s:%d", scheme, podIP, port)

	return &ClientInfo{
		BaseURL:   baseURL,
		Username:  string(username),
		Password:  string(password),
		Transport: transport,
	}, nil
}
