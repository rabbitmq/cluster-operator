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

	v1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// clusterOptions contains all configuration options for creating a RabbitmqCluster
type clusterOptions struct {
	// Basic configuration
	Replicas         int32
	Image            string
	ImagePullSecrets []string

	// Service configuration
	ServiceType        string
	ServiceAnnotations map[string]string

	// Persistence configuration
	StorageClassName string
	StorageSize      string

	// Resources configuration
	UnlimitedResources bool
	MemoryLimit        string
	MemoryRequest      string
	CPULimit           string
	CPURequest         string

	// TLS configuration
	TLSSecretName          string
	TLSCASecretName        string
	DisableNonTLSListeners bool

	// RabbitMQ configuration
	AdditionalPlugins    []string
	EnvConfig            map[string]string
	AdditionalConfigFile string
	AdvancedConfigFile   string

	// Other
	TerminationGracePeriodSeconds int64
	DelayStartSeconds             int32
	SkipPostDeploySteps           bool
	AutoEnableAllFeatureFlags     bool
}

// buildRabbitmqCluster constructs a RabbitmqCluster resource from the provided options
func buildRabbitmqCluster(name string, opts clusterOptions) (*v1beta1.RabbitmqCluster, error) {
	cluster := &v1beta1.RabbitmqCluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rabbitmq.com/v1beta1",
			Kind:       "RabbitmqCluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1beta1.RabbitmqClusterSpec{},
	}

	// Basic configuration
	if opts.Replicas > 0 {
		cluster.Spec.Replicas = &opts.Replicas
	}
	if opts.Image != "" {
		cluster.Spec.Image = opts.Image
	}
	if len(opts.ImagePullSecrets) > 0 {
		for _, secret := range opts.ImagePullSecrets {
			cluster.Spec.ImagePullSecrets = append(cluster.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: secret})
		}
	}

	// Service configuration
	if opts.ServiceType != "" {
		cluster.Spec.Service.Type = corev1.ServiceType(opts.ServiceType)
	}
	if len(opts.ServiceAnnotations) > 0 {
		cluster.Spec.Service.Annotations = opts.ServiceAnnotations
	}

	// Persistence configuration
	if opts.StorageClassName != "" {
		cluster.Spec.Persistence.StorageClassName = &opts.StorageClassName
	}
	if opts.StorageSize != "" {
		size, err := resource.ParseQuantity(opts.StorageSize)
		if err != nil {
			return nil, fmt.Errorf("invalid storage size %q: %w", opts.StorageSize, err)
		}
		cluster.Spec.Persistence.Storage = &size
	}

	// Resources configuration
	if opts.UnlimitedResources {
		cluster.Spec.Resources = &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{},
			Limits:   corev1.ResourceList{},
		}
	} else {
		// Build resource requirements from individual flags
		resources := &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{},
			Limits:   corev1.ResourceList{},
		}
		hasResources := false

		if opts.MemoryRequest != "" {
			memReq, err := resource.ParseQuantity(opts.MemoryRequest)
			if err != nil {
				return nil, fmt.Errorf("invalid memory request %q: %w", opts.MemoryRequest, err)
			}
			resources.Requests[corev1.ResourceMemory] = memReq
			hasResources = true
		}
		if opts.CPURequest != "" {
			cpuReq, err := resource.ParseQuantity(opts.CPURequest)
			if err != nil {
				return nil, fmt.Errorf("invalid cpu request %q: %w", opts.CPURequest, err)
			}
			resources.Requests[corev1.ResourceCPU] = cpuReq
			hasResources = true
		}
		if opts.MemoryLimit != "" {
			memLimit, err := resource.ParseQuantity(opts.MemoryLimit)
			if err != nil {
				return nil, fmt.Errorf("invalid memory limit %q: %w", opts.MemoryLimit, err)
			}
			resources.Limits[corev1.ResourceMemory] = memLimit
			hasResources = true
		}
		if opts.CPULimit != "" {
			cpuLimit, err := resource.ParseQuantity(opts.CPULimit)
			if err != nil {
				return nil, fmt.Errorf("invalid cpu limit %q: %w", opts.CPULimit, err)
			}
			resources.Limits[corev1.ResourceCPU] = cpuLimit
			hasResources = true
		}

		if hasResources {
			cluster.Spec.Resources = resources
		}
	}

	// TLS configuration
	if opts.TLSSecretName != "" {
		cluster.Spec.TLS.SecretName = opts.TLSSecretName
	}
	if opts.TLSCASecretName != "" {
		cluster.Spec.TLS.CaSecretName = opts.TLSCASecretName
	}
	if opts.DisableNonTLSListeners {
		cluster.Spec.TLS.DisableNonTLSListeners = true
	}

	// RabbitMQ configuration
	if len(opts.AdditionalPlugins) > 0 {
		for _, plugin := range opts.AdditionalPlugins {
			cluster.Spec.Rabbitmq.AdditionalPlugins = append(cluster.Spec.Rabbitmq.AdditionalPlugins, v1beta1.Plugin(plugin))
		}
	}

	// EnvConfig - convert map to rabbitmq-env.conf format
	if len(opts.EnvConfig) > 0 {
		var envLines []string
		for key, value := range opts.EnvConfig {
			envLines = append(envLines, fmt.Sprintf("%s=%s", key, value))
		}
		// Sort for deterministic output
		cluster.Spec.Rabbitmq.EnvConfig = joinSorted(envLines)
	}

	// Load additional config from file
	if opts.AdditionalConfigFile != "" {
		content, err := os.ReadFile(opts.AdditionalConfigFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read additional config file %q: %w", opts.AdditionalConfigFile, err)
		}
		cluster.Spec.Rabbitmq.AdditionalConfig = string(content)
	}

	// Load advanced config from file
	if opts.AdvancedConfigFile != "" {
		content, err := os.ReadFile(opts.AdvancedConfigFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read advanced config file %q: %w", opts.AdvancedConfigFile, err)
		}
		cluster.Spec.Rabbitmq.AdvancedConfig = string(content)
	}

	// Other configuration
	if opts.TerminationGracePeriodSeconds > 0 {
		cluster.Spec.TerminationGracePeriodSeconds = &opts.TerminationGracePeriodSeconds
	}
	if opts.DelayStartSeconds > 0 {
		cluster.Spec.DelayStartSeconds = &opts.DelayStartSeconds
	}
	if opts.SkipPostDeploySteps {
		cluster.Spec.SkipPostDeploySteps = true
	}
	if opts.AutoEnableAllFeatureFlags {
		cluster.Spec.AutoEnableAllFeatureFlags = true
	}

	return cluster, nil
}

// applyCluster marshals a RabbitmqCluster to YAML and applies it to the cluster
func applyCluster(executor *kubectlExecutor, cluster *v1beta1.RabbitmqCluster) error {
	yamlData, err := yaml.Marshal(cluster)
	if err != nil {
		return fmt.Errorf("failed to marshal cluster yaml: %w", err)
	}

	tmpfile, err := os.CreateTemp("", "rabbitmq-cluster-*.yaml")
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

// joinSorted joins strings with newlines, maintaining order for consistent output
func joinSorted(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}
