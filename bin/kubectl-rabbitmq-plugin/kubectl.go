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
	"os/exec"
	"strings"
)

// kubectlExecutor handles kubectl command execution with namespace configuration
type kubectlExecutor struct {
	namespace     string
	allNamespaces bool
}

// newKubectlExecutor creates a new kubectlExecutor with the given namespace configuration
func newKubectlExecutor(namespace string, allNamespaces bool) *kubectlExecutor {
	return &kubectlExecutor{
		namespace:     namespace,
		allNamespaces: allNamespaces,
	}
}

// Execute runs a kubectl command and streams output to stdout/stderr
func (k *kubectlExecutor) Execute(args ...string) error {
	cmdArgs := k.buildArgs(args...)
	if os.Getenv("KUBECTL_RABBITMQ_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "executing: kubectl %s\n", strings.Join(cmdArgs, " "))
	}

	cmd := exec.Command("kubectl", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// ExecuteWithOutput runs a kubectl command and returns its output
func (k *kubectlExecutor) ExecuteWithOutput(args ...string) ([]byte, error) {
	cmdArgs := k.buildArgs(args...)
	if os.Getenv("KUBECTL_RABBITMQ_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "executing: kubectl %s\n", strings.Join(cmdArgs, " "))
	}

	cmd := exec.Command("kubectl", cmdArgs...)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

// buildArgs constructs kubectl arguments with namespace flags placed correctly
func (k *kubectlExecutor) buildArgs(args ...string) []string {
	// For kubectl, namespace flags should come after the subcommand
	// e.g., "kubectl get --all-namespaces pods" not "kubectl --all-namespaces get pods"

	if len(args) == 0 {
		return args
	}

	// First argument is typically the kubectl subcommand (get, apply, etc.)
	cmdArgs := []string{args[0]}

	// Add namespace flags after the subcommand
	if k.allNamespaces {
		cmdArgs = append(cmdArgs, "--all-namespaces")
	} else if k.namespace != "" {
		cmdArgs = append(cmdArgs, "-n", k.namespace)
	}

	// Add remaining arguments
	if len(args) > 1 {
		cmdArgs = append(cmdArgs, args[1:]...)
	}

	return cmdArgs
}
