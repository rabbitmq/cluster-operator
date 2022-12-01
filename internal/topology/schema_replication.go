/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package internal

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"strings"
)

type UpstreamEndpoints struct {
	Username  string   `json:"username"`
	Password  string   `json:"password"`
	Endpoints []string `json:"endpoints"`
}

func GenerateSchemaReplicationParameters(secret *corev1.Secret, endpoints string) (UpstreamEndpoints, error) {
	username, ok := secret.Data["username"]
	if !ok {
		return UpstreamEndpoints{}, fmt.Errorf("could not find username in secret %s", secret.Name)
	}
	password, ok := secret.Data["password"]
	if !ok {
		return UpstreamEndpoints{}, fmt.Errorf("could not find password in secret %s", secret.Name)
	}

	if endpoints == "" {
		endpointsFromSecret, ok := secret.Data["endpoints"]
		if !ok {
			return UpstreamEndpoints{}, fmt.Errorf("could not find endpoints in secret %s or from spec.endpoints", secret.Name)
		}
		endpoints = string(endpointsFromSecret)
	}

	endpointsList := strings.Split(endpoints, ",")

	return UpstreamEndpoints{
		Username:  string(username),
		Password:  string(password),
		Endpoints: endpointsList,
	}, nil

}
