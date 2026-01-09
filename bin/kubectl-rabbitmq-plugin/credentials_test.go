// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCredentialsStruct(t *testing.T) {
	creds := &Credentials{
		Username: "testuser",
		Password: "testpass",
	}

	assert.Equal(t, "testuser", creds.Username)
	assert.Equal(t, "testpass", creds.Password)
}

// Note: GetInstanceCredentials requires actual kubectl access and K8s resources,
// so it's tested in the integration tests (commands_test.go) instead of here.
