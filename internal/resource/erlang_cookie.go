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
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/rabbitmq/cluster-operator/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	erlangCookieName = "erlang-cookie"
)

type ErlangCookieBuilder struct {
	*RabbitmqResourceBuilder
}

func (builder *RabbitmqResourceBuilder) ErlangCookie() *ErlangCookieBuilder {
	return &ErlangCookieBuilder{builder}
}

func (builder *ErlangCookieBuilder) Build() (client.Object, error) {
	cookie, err := randomEncodedString(24)
	if err != nil {
		return nil, err
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName(erlangCookieName),
			Namespace: builder.Instance.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			".erlang.cookie": []byte(cookie),
		},
	}, nil
}

func (builder *ErlangCookieBuilder) UpdateMayRequireStsRecreate() bool {
	return false
}

func (builder *ErlangCookieBuilder) Update(object client.Object) error {
	secret := object.(*corev1.Secret)
	secret.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)
	secret.Annotations = metadata.ReconcileAndFilterAnnotations(secret.GetAnnotations(), builder.Instance.Annotations)

	if err := controllerutil.SetControllerReference(builder.Instance, secret, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %w", err)
	}

	return nil
}

func randomEncodedString(dataLen int) (string, error) {
	randomBytes := make([]byte, dataLen)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(randomBytes), nil
}
