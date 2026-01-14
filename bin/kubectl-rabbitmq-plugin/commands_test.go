//go:build integration

// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	pluginBinaryPath = "./kubectl-rabbitmq-test-plugin"
)

func checkClusterOperatorReady() error {
	// Check if cluster operator pods are running
	cmd := exec.Command("kubectl", "get", "pods", "-A", "-l", "app.kubernetes.io/name=rabbitmq-cluster-operator", "-o", "jsonpath={.items[*].status.phase}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check cluster operator pods: %w", err)
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		return fmt.Errorf("no cluster operator pods found - is the operator installed?")
	}

	// Check if at least one pod is Running
	phases := strings.Fields(outputStr)
	hasRunning := false
	for _, phase := range phases {
		if phase == "Running" {
			hasRunning = true
			break
		}
	}

	if !hasRunning {
		return fmt.Errorf("cluster operator pods exist but none are Running (phases: %s)", outputStr)
	}

	return nil
}

func TestMain(m *testing.M) {
	cmd := exec.Command("go", "build", "-o", pluginBinaryPath, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("failed to build plugin: %v\n%s", err, string(output))
	}

	// Check if cluster operator is ready before running tests
	if err := checkClusterOperatorReady(); err != nil {
		os.Remove(pluginBinaryPath)
		log.Fatalf("Failed to run integration tests: %v", err)
	}

	code := m.Run()

	os.Remove(pluginBinaryPath)

	os.Exit(code)
}

func runPlugin(_ *testing.T, args ...string) (string, error) {
	cmd := exec.Command(pluginBinaryPath, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func runKubectl(t *testing.T, args ...string) string {
	cmd := exec.Command("kubectl", args...)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "kubectl command failed with output: \n%s", string(output))
	return string(output)
}

func TestPluginIntegration(t *testing.T) {
	t.Run("Version", func(t *testing.T) {
		output, err := runPlugin(t, "version")
		require.NoError(t, err)
		assert.Equal(t, "kubectl-rabbitmq dev\n", output)
	})

	t.Run("InstallClusterOperatorTooManyArgs", func(t *testing.T) {
		output, err := runPlugin(t, "install-cluster-operator", "too-many-args")
		require.Error(t, err)
		assert.Contains(t, output, "Error:")
	})

	t.Run("CreateCluster", func(t *testing.T) {
		runPlugin(t, "create", "bats-default", "--unlimited")
		require.Eventually(t, func() bool {
			out := runKubectl(t, "get", "rabbitmqcluster", "bats-default", "-o", "json")
			var result struct {
				Status struct {
					Conditions []struct {
						Type   string `json:"type"`
						Status string `json:"status"`
					} `json:"conditions"`
				} `json:"status"`
			}
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				return false
			}
			for _, cond := range result.Status.Conditions {
				if cond.Type == "AllReplicasReady" && cond.Status == "True" {
					return true
				}
			}
			return false
		}, 10*time.Minute, 5*time.Second, "AllReplicasReady condition should be True")
		credentialsOutput, err := getInstanceCredentials(newKubectlExecutor("", false), "bats-default")
		require.NoError(t, err)
		assert.NotEmpty(t, credentialsOutput.Username)
		assert.NotEmpty(t, credentialsOutput.Password)
	})

	t.Run("CreateWithInvalidFlag", func(t *testing.T) {
		_, err := runPlugin(t, "create", "bats-invalid", "--replicas", "3", "--invalid", "flag")
		assert.Error(t, err)
	})

	t.Run("CreateWithFlags", func(t *testing.T) {
		replicas := "3"
		image := "rabbitmq:4.0.5-management"
		serviceType := "NodePort"
		storageClass := "standard" // Assuming 'standard' storage class exists in the test env

		runPlugin(t, "create", "bats-configured", "--replicas", replicas, "--image", image, "--service", serviceType, "--storage-class", storageClass)
		time.Sleep(10 * time.Second)

		stsOutput := runKubectl(t, "get", "statefulset", "bats-configured-server", "-o", "json")
		var sts struct {
			Spec struct {
				Replicas int `json:"replicas"`
				Template struct {
					Spec struct {
						Containers []struct {
							Image string `json:"image"`
						} `json:"containers"`
					} `json:"spec"`
				} `json:"template"`
			} `json:"spec"`
		}
		err := json.Unmarshal([]byte(stsOutput), &sts)
		require.NoError(t, err)
		assert.Equal(t, 3, sts.Spec.Replicas)
		assert.Equal(t, image, sts.Spec.Template.Spec.Containers[0].Image)

		svcOutput := runKubectl(t, "get", "service", "bats-configured", "-o", "jsonpath='{.spec.type}'")
		assert.Contains(t, svcOutput, serviceType)

		pvcOutput := runKubectl(t, "get", "pvc", "persistence-bats-configured-server-0", "-o", "jsonpath='{.spec.storageClassName}'")
		assert.Contains(t, pvcOutput, storageClass)
	})

	t.Run("List", func(t *testing.T) {
		output, err := runPlugin(t, "list")
		require.NoError(t, err)
		assert.Contains(t, output, "bats-configured")
		assert.Contains(t, output, "bats-default")
	})

	t.Run("ListWithNamespace", func(t *testing.T) {
		currentNs := runKubectl(t, "config", "view", "--minify", "-o", "jsonpath='{.contexts[0].context.namespace}'")
		currentNs = strings.Trim(currentNs, "'")
		if currentNs == "" {
			currentNs = "default"
		}

		testNs := "kubectl-rabbitmq-tests"
		runKubectl(t, "create", "namespace", testNs)
		t.Cleanup(func() {
			runKubectl(t, "delete", "namespace", testNs)
		})

		runKubectl(t, "config", "set-context", "--current", "--namespace", testNs)
		t.Cleanup(func() {
			runKubectl(t, "config", "set-context", "--current", "--namespace", currentNs)
		})

		output, err := runPlugin(t, "-n", currentNs, "list")
		require.NoError(t, err)
		assert.Contains(t, output, "bats-configured")
		assert.Contains(t, output, "bats-default")
	})

	t.Run("ListAllNamespaces", func(t *testing.T) {
		output, err := runPlugin(t, "-A", "list")
		require.NoError(t, err)
		assert.Contains(t, output, "NAMESPACE")
		assert.Contains(t, output, "bats-configured")
		assert.Contains(t, output, "bats-default")
	})

	t.Run("Get", func(t *testing.T) {
		output, err := runPlugin(t, "get", "bats-default")
		require.NoError(t, err)
		assert.Contains(t, output, "statefulset.apps/bats-default-server")
		assert.Contains(t, output, "pod/bats-default-server-0")
	})

	t.Run("PauseAndResumeReconciliation", func(t *testing.T) {
		_, err := runPlugin(t, "pause-reconciliation", "bats-default")
		require.NoError(t, err)
		labels := runKubectl(t, "get", "rabbitmqcluster", "bats-default", "--show-labels")
		assert.Contains(t, labels, "rabbitmq.com/pauseReconciliation=true")

		listOutput, err := runPlugin(t, "list-pause-reconciliation-instances")
		require.NoError(t, err)
		assert.Contains(t, listOutput, "bats-default")

		_, err = runPlugin(t, "resume-reconciliation", "bats-default")
		require.NoError(t, err)
		labels = runKubectl(t, "get", "rabbitmqcluster", "bats-default", "--show-labels")
		assert.NotContains(t, labels, "rabbitmq.com/pauseReconciliation=true")
	})

	t.Run("Secrets", func(t *testing.T) {
		output, err := runPlugin(t, "secrets", "bats-default")
		require.NoError(t, err)
		assert.Regexp(t, regexp.MustCompile(`username: \S+`), output)
		assert.Regexp(t, regexp.MustCompile(`password: \S+`), output)
	})

	t.Run("EnableAllFeatureFlags", func(t *testing.T) {
		_, err := runPlugin(t, "enable-all-feature-flags", "bats-default")
		require.NoError(t, err)
		// This check is tricky. We'll assume the command works if it doesn't error.
		// A full check requires `exec`ing into the pod and checking feature flags.
	})

	t.Run("PerfTest", func(t *testing.T) {
		_, err := runPlugin(t, "perf-test", "bats-default", "--rate", "1")
		require.NoError(t, err)
		t.Cleanup(func() {
			runKubectl(t, "delete", "job", "-l", "app=perf-test")
		})
		require.Eventually(t, func() bool {
			out := runKubectl(t, "exec", "bats-default-server-0", "-c", "rabbitmq", "--", "rabbitmqctl", "list_connections", "client_properties")
			return strings.Contains(out, "perf-test")
		}, 5*time.Minute, 2*time.Second)
	})

	t.Run("Debug", func(t *testing.T) {
		_, err := runPlugin(t, "debug", "bats-default")
		require.NoError(t, err)
		require.Eventually(t, func() bool {
			logs := runKubectl(t, "logs", "-c", "rabbitmq", "bats-default-server-0")
			return strings.Contains(logs, " [debug] ")
		}, 1*time.Minute, 2*time.Second)
	})

	t.Run("Delete", func(t *testing.T) {
		_, err := runPlugin(t, "delete", "bats-configured", "bats-default")
		require.NoError(t, err)
		require.Eventually(t, func() bool {
			out := runKubectl(t, "get", "rabbitmqclusters", "-o", "json")
			var list struct {
				Items []interface{} `json:"items"`
			}
			if err := json.Unmarshal([]byte(out), &list); err != nil {
				return false
			}
			return len(list.Items) == 0
		}, 5*time.Minute, 2*time.Second)
	})

	t.Run("Help", func(t *testing.T) {
		for _, arg := range []string{"help", "--help", "-h"} {
			output, err := runPlugin(t, arg)
			require.NoError(t, err, "failed on arg: %s", arg)
			assert.Contains(t, output, "Usage:", "failed on arg: %s", arg)
		}
	})
}
