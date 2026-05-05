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

const (
	servicePortNameManagement    = "management"
	servicePortNameManagementTLS = "management-tls"
)

func readDefaultUserCredentials(ctx context.Context, k8sClient client.Reader, rmq *rabbitmqv1beta1.RabbitmqCluster) (username string, password string, err error) {
	secretName := rmq.ChildResourceName("default-user")
	secret := &corev1.Secret{}
	if err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      secretName,
		Namespace: rmq.Namespace,
	}, secret); err != nil {
		return "", "", fmt.Errorf("failed to get default user secret: %w", err)
	}

	u, ok := secret.Data["username"]
	if !ok {
		return "", "", fmt.Errorf("username not found in secret %s", secretName)
	}

	p, ok := secret.Data["password"]
	if !ok {
		return "", "", fmt.Errorf("password not found in secret %s", secretName)
	}

	return string(u), string(p), nil
}

// managementHTTPSTransport returns a transport configured for the management HTTPS listener when
// spec.tls.disableNonTLSListeners is true. serverName is used for TLS verification (SNI / cert SANs).
func managementHTTPSTransport(ctx context.Context, k8sClient client.Reader, rmq *rabbitmqv1beta1.RabbitmqCluster, serverName string) (*http.Transport, error) {
	if !rmq.Spec.TLS.DisableNonTLSListeners {
		return nil, nil
	}

	tlsConfig := &tls.Config{}

	certPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("failed to get system cert pool: %w", err)
	}

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

	tlsConfig.ServerName = serverName

	return &http.Transport{
		TLSClientConfig: tlsConfig,
	}, nil
}

func managementPortFromService(svc *corev1.Service, tlsOnly bool) (int32, error) {
	portName := servicePortNameManagement
	if tlsOnly {
		portName = servicePortNameManagementTLS
	}
	for i := range svc.Spec.Ports {
		if svc.Spec.Ports[i].Name == portName {
			return svc.Spec.Ports[i].Port, nil
		}
	}
	return 0, fmt.Errorf("port %q not found on service %s", portName, svc.Name)
}

// getClientInfoForPod creates ClientInfo for a specific pod using its stable DNS name.
// This is useful for checking individual pods instead of going through the service.
func getClientInfoForPod(ctx context.Context, k8sClient client.Reader, rmq *rabbitmqv1beta1.RabbitmqCluster, podName string) (*ClientInfo, error) {
	username, password, err := readDefaultUserCredentials(ctx, k8sClient, rmq)
	if err != nil {
		return nil, err
	}

	var port int
	var scheme string

	headlessServiceName := rmq.ChildResourceName("nodes")
	podFQDN := fmt.Sprintf("%s.%s.%s.svc", podName, headlessServiceName, rmq.Namespace)

	var transport *http.Transport
	if rmq.Spec.TLS.DisableNonTLSListeners {
		port = 15671
		scheme = "https"
		transport, err = managementHTTPSTransport(ctx, k8sClient, rmq, podFQDN)
		if err != nil {
			return nil, err
		}
	} else {
		port = 15672
		scheme = "http"
	}

	baseURL := fmt.Sprintf("%s://%s:%d", scheme, podFQDN, port)

	return &ClientInfo{
		BaseURL:   baseURL,
		Username:  username,
		Password:  password,
		Transport: transport,
	}, nil
}

// getClientInfoForService creates ClientInfo using the cluster's main Service DNS name (the Service
// that exposes the management UI), not the headless nodes Service.
func getClientInfoForService(ctx context.Context, k8sClient client.Reader, rmq *rabbitmqv1beta1.RabbitmqCluster) (*ClientInfo, error) {
	username, password, err := readDefaultUserCredentials(ctx, k8sClient, rmq)
	if err != nil {
		return nil, err
	}

	svcName := rmq.ChildResourceName("")
	svc := &corev1.Service{}
	if err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      svcName,
		Namespace: rmq.Namespace,
	}, svc); err != nil {
		return nil, fmt.Errorf("failed to get rabbitmq service: %w", err)
	}

	tlsOnly := rmq.Spec.TLS.DisableNonTLSListeners
	svcPort, err := managementPortFromService(svc, tlsOnly)
	if err != nil {
		return nil, err
	}

	serviceHost := fmt.Sprintf("%s.%s.svc", svc.Name, rmq.Namespace)

	var scheme string
	var transport *http.Transport
	if tlsOnly {
		scheme = "https"
		transport, err = managementHTTPSTransport(ctx, k8sClient, rmq, serviceHost)
		if err != nil {
			return nil, err
		}
	} else {
		scheme = "http"
	}

	baseURL := fmt.Sprintf("%s://%s:%d", scheme, serviceHost, svcPort)

	return &ClientInfo{
		BaseURL:   baseURL,
		Username:  username,
		Password:  password,
		Transport: transport,
	}, nil
}
