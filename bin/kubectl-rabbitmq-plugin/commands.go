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
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newInstallClusterOperatorCmd(getExecutor func() *kubectlExecutor) *cobra.Command {
	return &cobra.Command{
		Use:   "install-cluster-operator",
		Short: "Install the latest released RabbitMQ Cluster Operator",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := getExecutor()
			return executor.Execute("apply", "-f", "https://github.com/rabbitmq/cluster-operator/releases/latest/download/cluster-operator-ghcr-io.yml")
		},
	}
}

func newListCmd(getExecutor func() *kubectlExecutor) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all RabbitMQ clusters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := getExecutor()
			return executor.Execute("get", "rabbitmqclusters", "--sort-by={.metadata.name}")
		},
	}
}

func newCreateCmd(getExecutor func() *kubectlExecutor) *cobra.Command {
	var opts clusterOptions

	cmd := &cobra.Command{
		Use:   "create INSTANCE",
		Short: "Create a RabbitMQ cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := getExecutor()
			instanceName := args[0]

			cluster, err := buildRabbitmqCluster(instanceName, opts)
			if err != nil {
				return fmt.Errorf("failed to build cluster: %w", err)
			}

			return applyCluster(executor, cluster)
		},
	}

	// Basic configuration
	cmd.Flags().Int32Var(&opts.Replicas, "replicas", 1, "Number of replicas")
	cmd.Flags().StringVar(&opts.Image, "image", "", "RabbitMQ image")
	cmd.Flags().StringSliceVar(&opts.ImagePullSecrets, "image-pull-secret", []string{}, "Image pull secret (repeatable)")

	// Service configuration
	cmd.Flags().StringVar(&opts.ServiceType, "service", "ClusterIP", "Service type (ClusterIP, LoadBalancer, NodePort)")
	cmd.Flags().StringToStringVar(&opts.ServiceAnnotations, "service-annotation", map[string]string{}, "Service annotation in key=value format (repeatable)")

	// Persistence configuration
	cmd.Flags().StringVar(&opts.StorageClassName, "storage-class", "", "Storage class name")
	cmd.Flags().StringVar(&opts.StorageSize, "storage-size", "10Gi", "Storage size")

	// Resources configuration
	cmd.Flags().BoolVar(&opts.UnlimitedResources, "unlimited", false, "Set unlimited resources (empty requests and limits)")
	cmd.Flags().StringVar(&opts.MemoryLimit, "memory-limit", "", "Memory limit (e.g., 2Gi)")
	cmd.Flags().StringVar(&opts.MemoryRequest, "memory-request", "", "Memory request (e.g., 2Gi)")
	cmd.Flags().StringVar(&opts.CPULimit, "cpu-limit", "", "CPU limit (e.g., 2000m)")
	cmd.Flags().StringVar(&opts.CPURequest, "cpu-request", "", "CPU request (e.g., 1000m)")

	// TLS configuration
	cmd.Flags().StringVar(&opts.TLSSecretName, "tls-secret", "", "TLS secret name")
	cmd.Flags().StringVar(&opts.TLSCASecretName, "tls-ca-secret", "", "TLS CA secret name")
	cmd.Flags().BoolVar(&opts.DisableNonTLSListeners, "disable-non-tls-listeners", false, "Disable non-TLS listeners")

	// RabbitMQ configuration
	cmd.Flags().StringSliceVar(&opts.AdditionalPlugins, "plugin", []string{}, "Additional plugin to enable (repeatable)")
	cmd.Flags().StringToStringVar(&opts.EnvConfig, "env-conf", map[string]string{}, "Environment configuration in key=value format (repeatable)")
	cmd.Flags().StringVar(&opts.AdditionalConfigFile, "additional-config-file", "", "Path to additional config file")
	cmd.Flags().StringVar(&opts.AdvancedConfigFile, "advanced-config-file", "", "Path to advanced config file")

	// Other configuration
	cmd.Flags().Int64Var(&opts.TerminationGracePeriodSeconds, "termination-grace-period", 0, "Termination grace period in seconds")
	cmd.Flags().Int32Var(&opts.DelayStartSeconds, "delay-start", 0, "Delay start in seconds")
	cmd.Flags().BoolVar(&opts.SkipPostDeploySteps, "skip-post-deploy-steps", false, "Skip post deploy steps")
	cmd.Flags().BoolVar(&opts.AutoEnableAllFeatureFlags, "auto-enable-all-feature-flags", false, "Automatically enable all feature flags")

	return cmd
}

func newDeleteCmd(getExecutor func() *kubectlExecutor) *cobra.Command {
	return &cobra.Command{
		Use:   "delete INSTANCE [INSTANCE...]",
		Short: "Delete one or more RabbitMQ clusters",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := getExecutor()
			for _, cluster := range args {
				if err := executor.Execute("delete", "rabbitmqcluster", cluster); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newGetCmd(getExecutor func() *kubectlExecutor) *cobra.Command {
	return &cobra.Command{
		Use:   "get INSTANCE",
		Short: "Get a RabbitMQ cluster and dependent objects",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := getExecutor()
			instance := args[0]
			return executor.Execute("get", "pods,cm,sts,svc,secrets,rs", "-l", fmt.Sprintf("app.kubernetes.io/name=%s", instance))
		},
	}
}

func newDebugCmd(getExecutor func() *kubectlExecutor) *cobra.Command {
	return &cobra.Command{
		Use:   "debug INSTANCE",
		Short: "Set log level to 'debug' on all nodes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := getExecutor()
			instance := args[0]

			// Get list of pods
			output, err := executor.ExecuteWithOutput("get", "pods", "-l", fmt.Sprintf("app.kubernetes.io/name=%s", instance), "-o", "custom-columns=name:.metadata.name", "--no-headers")
			if err != nil {
				return err
			}

			pods := strings.Split(strings.TrimSpace(string(output)), "\n")
			for _, pod := range pods {
				if pod == "" {
					continue
				}
				fmt.Printf("%s: ", pod)
				if err := executor.Execute("exec", pod, "-c", "rabbitmq", "--", "rabbitmqctl", "set_log_level", "debug"); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newTailCmd(getExecutor func() *kubectlExecutor) *cobra.Command {
	return &cobra.Command{
		Use:   "tail INSTANCE",
		Short: "Tail logs from all nodes (requires kubectl tail plugin)",
		Long:  "Tail logs from all nodes. Requires the 'tail' plugin. Install it with 'kubectl krew install tail'",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := getExecutor()
			instance := args[0]
			return executor.Execute("tail", "--svc", instance)
		},
	}
}

func newObserveCmd(getExecutor func() *kubectlExecutor) *cobra.Command {
	return &cobra.Command{
		Use:   "observe INSTANCE NODE",
		Short: "Run 'rabbitmq-diagnostics observer' on a specific node",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := getExecutor()
			instance := args[0]
			node := args[1]
			podName := fmt.Sprintf("%s-server-%s", instance, node)
			return executor.Execute("exec", "-it", podName, "-c", "rabbitmq", "--", "rabbitmq-diagnostics", "observer")
		},
	}
}

func newSecretsCmd(getExecutor func() *kubectlExecutor) *cobra.Command {
	return &cobra.Command{
		Use:   "secrets INSTANCE",
		Short: "Print default-user credentials for an instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := getExecutor()
			instance := args[0]

			creds, err := getInstanceCredentials(executor, instance)
			if err != nil {
				return err
			}

			fmt.Printf("username: %s\n", creds.Username)
			fmt.Printf("password: %s\n", creds.Password)
			return nil
		},
	}
}

func newEnableAllFeatureFlagsCmd(getExecutor func() *kubectlExecutor) *cobra.Command {
	return &cobra.Command{
		Use:   "enable-all-feature-flags INSTANCE",
		Short: "Enable all feature flags on an instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := getExecutor()
			instance := args[0]
			podName := fmt.Sprintf("%s-server-0", instance)
			return executor.Execute("exec", podName, "-c", "rabbitmq", "--", "rabbitmqctl", "enable_feature_flag", "all")
		},
	}
}

func newPauseReconciliationCmd(getExecutor func() *kubectlExecutor) *cobra.Command {
	return &cobra.Command{
		Use:   "pause-reconciliation INSTANCE",
		Short: "Pause reconciliation for an instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := getExecutor()
			instance := args[0]
			return executor.Execute("label", "rabbitmqclusters", instance, "rabbitmq.com/pauseReconciliation=true")
		},
	}
}

func newResumeReconciliationCmd(getExecutor func() *kubectlExecutor) *cobra.Command {
	return &cobra.Command{
		Use:   "resume-reconciliation INSTANCE",
		Short: "Resume reconciliation for an instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := getExecutor()
			instance := args[0]
			return executor.Execute("label", "rabbitmqclusters", instance, "rabbitmq.com/pauseReconciliation-")
		},
	}
}

func newListPauseReconciliationInstancesCmd(getExecutor func() *kubectlExecutor) *cobra.Command {
	return &cobra.Command{
		Use:   "list-pause-reconciliation-instances",
		Short: "List all instances that have the pause reconciliation label",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := getExecutor()
			return executor.Execute("get", "rabbitmqclusters", "-l", "rabbitmq.com/pauseReconciliation=true", "--show-labels")
		},
	}
}

func newManageCmd(getExecutor func() *kubectlExecutor) *cobra.Command {
	return &cobra.Command{
		Use:   "manage INSTANCE",
		Short: "Open Management UI for an instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := getExecutor()
			instance := args[0]
			serviceName := instance

			// Check if TLS is enabled
			output, err := executor.ExecuteWithOutput("get", "service", serviceName, "-o", "jsonpath={.spec.ports[?(@.name==\"management-tls\")]}")
			if err != nil {
				return err
			}

			var mgmtPort, mgmtURL string
			if len(output) > 0 {
				mgmtPort = "15671"
				mgmtURL = "https://localhost:15671/"
			} else {
				mgmtPort = "15672"
				mgmtURL = "http://localhost:15672/"
			}

			// Determine the open command based on OS
			var openCmd string
			switch runtime.GOOS {
			case "darwin":
				openCmd = "open"
			case "linux":
				openCmd = "xdg-open"
			default:
				fmt.Printf("Please open your browser to %s\n", mgmtURL)
			}

			// Open browser in background
			if openCmd != "" {
				go func() {
					time.Sleep(2 * time.Second)
					// Ignore error from browser open
					_ = executor.Execute(openCmd, mgmtURL)
				}()
			}

			// Start port-forward (blocking)
			return executor.Execute("port-forward", fmt.Sprintf("service/%s", serviceName), mgmtPort)
		},
	}
}

func newPerfTestCmd(getExecutor func() *kubectlExecutor) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "perf-test INSTANCE [PERF_TEST_ARGS...]",
		Short: "Create a Job to run perf-test against an instance",
		Long: `Create a Job to run perf-test against an instance. You can pass any perf-test parameters.
See https://rabbitmq.github.io/rabbitmq-perf-test/stable/htmlsingle/ for more details.

To monitor perf-test, create a PodMonitor:
  apiVersion: monitoring.coreos.com/v1
  kind: PodMonitor
  metadata:
    name: kubectl-perf-test
  spec:
    podMetricsEndpoints:
    - interval: 15s
      port: prometheus
    selector:
      matchLabels:
        app: perf-test`,
		Args:                  cobra.MinimumNArgs(1),
		DisableFlagParsing:    true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := getExecutor()
			instance := args[0]
			perfTestArgs := args[1:]

			creds, err := getInstanceCredentials(executor, instance)
			if err != nil {
				return err
			}

			return createPerfTestJob(executor, instance, creds, perfTestArgs)
		},
	}
	return cmd
}

func newStreamPerfTestCmd(getExecutor func() *kubectlExecutor) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stream-perf-test INSTANCE [STREAM_PERF_TEST_ARGS...]",
		Short: "Create a Job to run stream-perf-test against an instance",
		Long: `Create a Job to run stream-perf-test against an instance. You can pass any stream perf-test parameters.
See https://rabbitmq.github.io/rabbitmq-stream-java-client/snapshot/htmlsingle/ for more details.`,
		Args:                  cobra.MinimumNArgs(1),
		DisableFlagParsing:    true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := getExecutor()
			instance := args[0]
			streamPerfTestArgs := args[1:]

			creds, err := getInstanceCredentials(executor, instance)
			if err != nil {
				return err
			}

			return createStreamPerfTestJob(executor, instance, creds, streamPerfTestArgs)
		},
	}
	return cmd
}

func newVersionCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print kubectl-rabbitmq plugin version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("kubectl-rabbitmq %s\n", version)
		},
	}
}
