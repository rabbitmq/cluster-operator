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

func TestNewKubectlExecutor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		namespace     string
		allNamespaces bool
	}{
		{
			name:          "with namespace",
			namespace:     "my-namespace",
			allNamespaces: false,
		},
		{
			name:          "with all namespaces",
			namespace:     "",
			allNamespaces: true,
		},
		{
			name:          "no namespace flags",
			namespace:     "",
			allNamespaces: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			executor := newKubectlExecutor(tt.namespace, tt.allNamespaces)
			assert.NotNil(t, executor)
			assert.Equal(t, tt.namespace, executor.namespace)
			assert.Equal(t, tt.allNamespaces, executor.allNamespaces)
		})
	}
}

func TestBuildArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		namespace     string
		allNamespaces bool
		args          []string
		expected      []string
	}{
		{
			name:          "with namespace",
			namespace:     "my-namespace",
			allNamespaces: false,
			args:          []string{"get", "pods"},
			expected:      []string{"get", "-n", "my-namespace", "pods"},
		},
		{
			name:          "with all namespaces",
			namespace:     "",
			allNamespaces: true,
			args:          []string{"get", "pods"},
			expected:      []string{"get", "--all-namespaces", "pods"},
		},
		{
			name:          "no namespace flags",
			namespace:     "",
			allNamespaces: false,
			args:          []string{"get", "pods"},
			expected:      []string{"get", "pods"},
		},
		{
			name:          "single arg with namespace",
			namespace:     "test",
			allNamespaces: false,
			args:          []string{"version"},
			expected:      []string{"version", "-n", "test"},
		},
		{
			name:          "empty args",
			namespace:     "test",
			allNamespaces: false,
			args:          []string{},
			expected:      []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			executor := newKubectlExecutor(tt.namespace, tt.allNamespaces)
			result := executor.buildArgs(tt.args...)
			assert.Equal(t, tt.expected, result)
		})
	}
}
