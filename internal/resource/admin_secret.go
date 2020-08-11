// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
//

package resource

import (
	"fmt"
	"net/url"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	AdminSecretName = "admin"
	BindingType     = "rabbitmq"
	BindingProvider = "rabbitmq-cluster-operator"
)

type AdminSecretBuilder struct {
	Instance *rabbitmqv1beta1.RabbitmqCluster
}

func (builder *RabbitmqResourceBuilder) AdminSecret() *AdminSecretBuilder {
	return &AdminSecretBuilder{
		Instance: builder.Instance,
	}
}

func (builder *AdminSecretBuilder) Update(object runtime.Object) error {
	secret := object.(*corev1.Secret)
	secret.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)
	secret.Annotations = metadata.ReconcileAndFilterAnnotations(secret.GetAnnotations(), builder.Instance.Annotations)
	return nil
}

func (builder *AdminSecretBuilder) Build() (runtime.Object, error) {
	username, err := randomEncodedString(24)
	if err != nil {
		return nil, err
	}

	password, err := randomEncodedString(24)
	if err != nil {
		return nil, err
	}

	port := "5672"
	scheme := "amqp"
	if builder.Instance.TLSEnabled() {
		// TODO configure for TLS
		// port = "5671"
		// scheme = "amqps"
	}

	host := fmt.Sprintf("%s.%s.svc", builder.Instance.ChildResourceName("client"), builder.Instance.Namespace)
	uri := url.URL{
		Scheme: scheme,
		User:   url.UserPassword(username, password),
		Host:   fmt.Sprintf("%s:%s", host, port),
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName(AdminSecretName),
			Namespace: builder.Instance.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"type":     []byte(BindingType),
			"provider": []byte(BindingProvider),
			"scheme":   []byte(scheme),
			"host":     []byte(host),
			"port":     []byte(port),
			"username": []byte(username),
			"password": []byte(password),
			"uri":      []byte(uri.String()),
		},
	}, nil
}
