// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.

package main

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	// Plugin version
	pluginVersion = "dev"
)

func main() {
	var namespace string
	var allNamespaces bool

	rootCmd := &cobra.Command{
		Use:   "kubectl-rabbitmq",
		Short: "kubectl plugin for managing RabbitMQ clusters",
		Long: `kubectl-rabbitmq is a kubectl plugin for managing RabbitMQ clusters.
It provides commands to create, list, manage, and debug RabbitMQ clusters running on Kubernetes.`,
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace")
	rootCmd.PersistentFlags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "List resources across all namespaces")

	// Create a function to get the executor based on current flag values
	getExecutor := func() *kubectlExecutor {
		return newKubectlExecutor(namespace, allNamespaces)
	}

	// Add all subcommands - they will use getExecutor() to get the properly configured executor
	rootCmd.AddCommand(
		newInstallClusterOperatorCmd(getExecutor),
		newListCmd(getExecutor),
		newCreateCmd(getExecutor),
		newDeleteCmd(getExecutor),
		newGetCmd(getExecutor),
		newDebugCmd(getExecutor),
		newTailCmd(getExecutor),
		newObserveCmd(getExecutor),
		newSecretsCmd(getExecutor),
		newEnableAllFeatureFlagsCmd(getExecutor),
		newPauseReconciliationCmd(getExecutor),
		newResumeReconciliationCmd(getExecutor),
		newListPauseReconciliationInstancesCmd(getExecutor),
		newManageCmd(getExecutor),
		newPerfTestCmd(getExecutor),
		newStreamPerfTestCmd(getExecutor),
		newVersionCmd(pluginVersion),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
