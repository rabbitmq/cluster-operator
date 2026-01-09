// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.

package main

import (
	"encoding/base64"
	"fmt"
)

// Credentials holds RabbitMQ username and password
type Credentials struct {
	Username string
	Password string
}

// GetInstanceCredentials fetches the default user credentials for a RabbitMQ instance
func GetInstanceCredentials(executor *KubectlExecutor, instanceName string) (*Credentials, error) {
	secretName := fmt.Sprintf("%s-default-user", instanceName)

	// Get username
	usernameB64, err := executor.ExecuteWithOutput("get", "secret", secretName, "-o", "jsonpath={.data.username}")
	if err != nil {
		return nil, fmt.Errorf("failed to get username from secret: %w", err)
	}

	username, err := base64.StdEncoding.DecodeString(string(usernameB64))
	if err != nil {
		return nil, fmt.Errorf("failed to decode username: %w", err)
	}

	// Get password
	passwordB64, err := executor.ExecuteWithOutput("get", "secret", secretName, "-o", "jsonpath={.data.password}")
	if err != nil {
		return nil, fmt.Errorf("failed to get password from secret: %w", err)
	}

	password, err := base64.StdEncoding.DecodeString(string(passwordB64))
	if err != nil {
		return nil, fmt.Errorf("failed to decode password: %w", err)
	}

	return &Credentials{
		Username: string(username),
		Password: string(password),
	}, nil
}
