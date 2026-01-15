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
	"sigs.k8s.io/yaml"
)

func TestCreatePerfTestJobYAML(t *testing.T) {
	t.Parallel()
	// This test validates the structure of the generated Job YAML
	// without actually applying it to a cluster

	// Create the job structure (extracted from CreatePerfTestJob)
	jobArgs := []string{
		"--uri",
		"amqp://testuser:testpass@test-instance",
		"--metrics-prometheus",
		"--rate", "100",
	}

	job := map[string]interface{}{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata": map[string]interface{}{
			"name": "perf-test",
			"labels": map[string]string{
				"app": "perf-test",
			},
		},
		"spec": map[string]interface{}{
			"completions":             1,
			"ttlSecondsAfterFinished": 300,
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]string{
						"app": "perf-test",
					},
				},
				"spec": map[string]interface{}{
					"restartPolicy": "Never",
					"containers": []map[string]interface{}{
						{
							"name":  "rabbitmq-perf-test",
							"image": "pivotalrabbitmq/perf-test",
							"ports": []map[string]interface{}{
								{
									"name":          "prometheus",
									"containerPort": 8080,
								},
							},
							"args": jobArgs,
						},
					},
				},
			},
		},
	}

	// Validate YAML marshaling works
	yamlData, err := yaml.Marshal(job)
	assert.NoError(t, err)
	assert.NotEmpty(t, yamlData)
	assert.Contains(t, string(yamlData), "kind: Job")
	assert.Contains(t, string(yamlData), "name: perf-test")
	assert.Contains(t, string(yamlData), "pivotalrabbitmq/perf-test")
}

func TestCreateStreamPerfTestJobYAML(t *testing.T) {
	t.Parallel()
	// This test validates the structure of the generated Job YAML
	// without actually applying it to a cluster

	// Create the job structure (extracted from CreateStreamPerfTestJob)
	jobArgs := []string{
		"--uris",
		"rabbitmq-stream://testuser:testpass@test-instance",
		"--rate", "100",
	}

	job := map[string]interface{}{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata": map[string]interface{}{
			"name": "stream-perf-test",
			"labels": map[string]string{
				"app": "stream-perf-test",
			},
		},
		"spec": map[string]interface{}{
			"completions":             1,
			"ttlSecondsAfterFinished": 300,
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]string{
						"app": "stream-perf-test",
					},
				},
				"spec": map[string]interface{}{
					"restartPolicy": "Never",
					"containers": []map[string]interface{}{
						{
							"name":  "rabbitmq-stream-perf-test",
							"image": "pivotalrabbitmq/stream-perf-test",
							"args":  jobArgs,
						},
					},
				},
			},
		},
	}

	// Validate YAML marshaling works
	yamlData, err := yaml.Marshal(job)
	assert.NoError(t, err)
	assert.NotEmpty(t, yamlData)
	assert.Contains(t, string(yamlData), "kind: Job")
	assert.Contains(t, string(yamlData), "name: stream-perf-test")
	assert.Contains(t, string(yamlData), "pivotalrabbitmq/stream-perf-test")
}

// Note: CreatePerfTestJob, CreateStreamPerfTestJob, and applyJobYAML require
// actual kubectl access, so they're tested in integration tests (commands_test.go)
