// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

// CreatePerfTestJob creates and applies a perf-test Job to the cluster
func CreatePerfTestJob(executor *KubectlExecutor, instance string, creds *Credentials, args []string) error {
	jobArgs := []string{
		"--uri",
		fmt.Sprintf("amqp://%s:%s@%s", creds.Username, creds.Password, instance),
		"--metrics-prometheus",
	}
	jobArgs = append(jobArgs, args...)

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

	return applyJobYAML(executor, job)
}

// CreateStreamPerfTestJob creates and applies a stream-perf-test Job to the cluster
func CreateStreamPerfTestJob(executor *KubectlExecutor, instance string, creds *Credentials, args []string) error {
	jobArgs := []string{
		"--uris",
		fmt.Sprintf("rabbitmq-stream://%s:%s@%s", creds.Username, creds.Password, instance),
	}
	jobArgs = append(jobArgs, args...)

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

	return applyJobYAML(executor, job)
}

// applyJobYAML marshals a job definition to YAML and applies it to the cluster
func applyJobYAML(executor *KubectlExecutor, job map[string]interface{}) error {
	yamlData, err := yaml.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job yaml: %w", err)
	}

	tmpfile, err := os.CreateTemp("", "perf-test-job-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write(yamlData); err != nil {
		tmpfile.Close()
		return fmt.Errorf("failed to write yaml to temp file: %w", err)
	}
	if err := tmpfile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	return executor.Execute("apply", "-f", tmpfile.Name())
}
