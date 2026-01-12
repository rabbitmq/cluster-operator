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
	"testing"

	v1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"
)

func TestBuildRabbitmqCluster(t *testing.T) {
	t.Parallel()
	t.Run("minimal configuration", func(t *testing.T) {
		t.Parallel()
		opts := ClusterOptions{}
		cluster, err := BuildRabbitmqCluster("test-cluster", opts)

		require.NoError(t, err)
		assert.Equal(t, "test-cluster", cluster.Name)
		assert.Equal(t, "rabbitmq.com/v1beta1", cluster.APIVersion)
		assert.Equal(t, "RabbitmqCluster", cluster.Kind)
	})

	t.Run("with replicas", func(t *testing.T) {
		t.Parallel()
		opts := ClusterOptions{
			Replicas: 3,
		}
		cluster, err := BuildRabbitmqCluster("test-cluster", opts)

		require.NoError(t, err)
		require.NotNil(t, cluster.Spec.Replicas)
		assert.Equal(t, int32(3), *cluster.Spec.Replicas)
	})

	t.Run("with image and pull secrets", func(t *testing.T) {
		t.Parallel()
		opts := ClusterOptions{
			Image:            "rabbitmq:3.12",
			ImagePullSecrets: []string{"secret1", "secret2"},
		}
		cluster, err := BuildRabbitmqCluster("test-cluster", opts)

		require.NoError(t, err)
		assert.Equal(t, "rabbitmq:3.12", cluster.Spec.Image)
		assert.Len(t, cluster.Spec.ImagePullSecrets, 2)
		assert.Equal(t, "secret1", cluster.Spec.ImagePullSecrets[0].Name)
		assert.Equal(t, "secret2", cluster.Spec.ImagePullSecrets[1].Name)
	})

	t.Run("with service configuration", func(t *testing.T) {
		t.Parallel()
		opts := ClusterOptions{
			ServiceType: "LoadBalancer",
			ServiceAnnotations: map[string]string{
				"service.beta.kubernetes.io/aws-load-balancer-type": "nlb",
			},
		}
		cluster, err := BuildRabbitmqCluster("test-cluster", opts)

		require.NoError(t, err)
		assert.Equal(t, corev1.ServiceType("LoadBalancer"), cluster.Spec.Service.Type)
		assert.Equal(t, "nlb", cluster.Spec.Service.Annotations["service.beta.kubernetes.io/aws-load-balancer-type"])
	})

	t.Run("with storage configuration", func(t *testing.T) {
		t.Parallel()
		storageClass := "fast-ssd"
		opts := ClusterOptions{
			StorageClassName: storageClass,
			StorageSize:      "20Gi",
		}
		cluster, err := BuildRabbitmqCluster("test-cluster", opts)

		require.NoError(t, err)
		assert.Equal(t, storageClass, *cluster.Spec.Persistence.StorageClassName)
		expectedSize := resource.MustParse("20Gi")
		assert.True(t, expectedSize.Equal(*cluster.Spec.Persistence.Storage))
	})

	t.Run("with invalid storage size", func(t *testing.T) {
		t.Parallel()
		opts := ClusterOptions{
			StorageSize: "invalid",
		}
		_, err := BuildRabbitmqCluster("test-cluster", opts)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid storage size")
	})

	t.Run("with unlimited resources", func(t *testing.T) {
		t.Parallel()
		opts := ClusterOptions{
			UnlimitedResources: true,
		}
		cluster, err := BuildRabbitmqCluster("test-cluster", opts)

		require.NoError(t, err)
		require.NotNil(t, cluster.Spec.Resources)
		assert.Empty(t, cluster.Spec.Resources.Requests)
		assert.Empty(t, cluster.Spec.Resources.Limits)
	})

	t.Run("with resource limits and requests", func(t *testing.T) {
		t.Parallel()
		opts := ClusterOptions{
			MemoryLimit:   "2Gi",
			MemoryRequest: "1Gi",
			CPULimit:      "2000m",
			CPURequest:    "1000m",
		}
		cluster, err := BuildRabbitmqCluster("test-cluster", opts)

		require.NoError(t, err)
		require.NotNil(t, cluster.Spec.Resources)

		expectedMemLimit := resource.MustParse("2Gi")
		assert.True(t, expectedMemLimit.Equal(cluster.Spec.Resources.Limits[corev1.ResourceMemory]))

		expectedMemReq := resource.MustParse("1Gi")
		assert.True(t, expectedMemReq.Equal(cluster.Spec.Resources.Requests[corev1.ResourceMemory]))

		expectedCPULimit := resource.MustParse("2000m")
		assert.True(t, expectedCPULimit.Equal(cluster.Spec.Resources.Limits[corev1.ResourceCPU]))

		expectedCPUReq := resource.MustParse("1000m")
		assert.True(t, expectedCPUReq.Equal(cluster.Spec.Resources.Requests[corev1.ResourceCPU]))
	})

	t.Run("with TLS configuration", func(t *testing.T) {
		t.Parallel()
		opts := ClusterOptions{
			TLSSecretName:          "tls-secret",
			TLSCASecretName:        "ca-secret",
			DisableNonTLSListeners: true,
		}
		cluster, err := BuildRabbitmqCluster("test-cluster", opts)

		require.NoError(t, err)
		assert.Equal(t, "tls-secret", cluster.Spec.TLS.SecretName)
		assert.Equal(t, "ca-secret", cluster.Spec.TLS.CaSecretName)
		assert.True(t, cluster.Spec.TLS.DisableNonTLSListeners)
	})

	t.Run("with additional plugins", func(t *testing.T) {
		t.Parallel()
		opts := ClusterOptions{
			AdditionalPlugins: []string{"rabbitmq_shovel", "rabbitmq_federation"},
		}
		cluster, err := BuildRabbitmqCluster("test-cluster", opts)

		require.NoError(t, err)
		assert.Len(t, cluster.Spec.Rabbitmq.AdditionalPlugins, 2)
		assert.Contains(t, cluster.Spec.Rabbitmq.AdditionalPlugins, v1beta1.Plugin("rabbitmq_shovel"))
		assert.Contains(t, cluster.Spec.Rabbitmq.AdditionalPlugins, v1beta1.Plugin("rabbitmq_federation"))
	})

	t.Run("with env config", func(t *testing.T) {
		t.Parallel()
		opts := ClusterOptions{
			EnvConfig: map[string]string{
				"RABBITMQ_SERVER_ADDITIONAL_ERL_ARGS": "-rabbit log_levels [{connection,debug}]",
				"ERL_MAX_PORTS":                       "4096",
			},
		}
		cluster, err := BuildRabbitmqCluster("test-cluster", opts)

		require.NoError(t, err)
		assert.Contains(t, cluster.Spec.Rabbitmq.EnvConfig, "RABBITMQ_SERVER_ADDITIONAL_ERL_ARGS=")
		assert.Contains(t, cluster.Spec.Rabbitmq.EnvConfig, "ERL_MAX_PORTS=4096")
	})

	t.Run("with config files", func(t *testing.T) {
		t.Parallel()
		// Create temporary config files
		additionalConfigContent := "log.console.level = debug\n"
		additionalConfigFile, err := os.CreateTemp("", "additional-config-*.conf")
		require.NoError(t, err)
		defer os.Remove(additionalConfigFile.Name())
		_, err = additionalConfigFile.WriteString(additionalConfigContent)
		require.NoError(t, err)
		additionalConfigFile.Close()

		advancedConfigContent := "[{rabbit, [{tcp_listeners, [5672]}]}].\n"
		advancedConfigFile, err := os.CreateTemp("", "advanced-config-*.config")
		require.NoError(t, err)
		defer os.Remove(advancedConfigFile.Name())
		_, err = advancedConfigFile.WriteString(advancedConfigContent)
		require.NoError(t, err)
		advancedConfigFile.Close()

		opts := ClusterOptions{
			AdditionalConfigFile: additionalConfigFile.Name(),
			AdvancedConfigFile:   advancedConfigFile.Name(),
		}
		cluster, err := BuildRabbitmqCluster("test-cluster", opts)

		require.NoError(t, err)
		assert.Equal(t, additionalConfigContent, cluster.Spec.Rabbitmq.AdditionalConfig)
		assert.Equal(t, advancedConfigContent, cluster.Spec.Rabbitmq.AdvancedConfig)
	})

	t.Run("with invalid config file", func(t *testing.T) {
		t.Parallel()
		opts := ClusterOptions{
			AdditionalConfigFile: "/nonexistent/file.conf",
		}
		_, err := BuildRabbitmqCluster("test-cluster", opts)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read additional config file")
	})

	t.Run("with other configuration", func(t *testing.T) {
		t.Parallel()
		opts := ClusterOptions{
			TerminationGracePeriodSeconds: 604800,
			DelayStartSeconds:             30,
			SkipPostDeploySteps:           true,
			AutoEnableAllFeatureFlags:     true,
		}
		cluster, err := BuildRabbitmqCluster("test-cluster", opts)

		require.NoError(t, err)
		require.NotNil(t, cluster.Spec.TerminationGracePeriodSeconds)
		assert.Equal(t, int64(604800), *cluster.Spec.TerminationGracePeriodSeconds)
		require.NotNil(t, cluster.Spec.DelayStartSeconds)
		assert.Equal(t, int32(30), *cluster.Spec.DelayStartSeconds)
		assert.True(t, cluster.Spec.SkipPostDeploySteps)
		assert.True(t, cluster.Spec.AutoEnableAllFeatureFlags)
	})

	t.Run("marshals to valid YAML", func(t *testing.T) {
		t.Parallel()
		opts := ClusterOptions{
			Replicas: 1,
			Image:    "rabbitmq:3.12",
		}
		cluster, err := BuildRabbitmqCluster("test-cluster", opts)
		require.NoError(t, err)

		yamlData, err := yaml.Marshal(cluster)
		require.NoError(t, err)
		assert.Contains(t, string(yamlData), "kind: RabbitmqCluster")
		assert.Contains(t, string(yamlData), "name: test-cluster")
		assert.Contains(t, string(yamlData), "rabbitmq:3.12")
	})
}

func TestJoinSorted(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		lines    []string
		expected string
	}{
		{
			name:     "empty slice",
			lines:    []string{},
			expected: "",
		},
		{
			name:     "single line",
			lines:    []string{"line1"},
			expected: "line1",
		},
		{
			name:     "multiple lines",
			lines:    []string{"line1", "line2", "line3"},
			expected: "line1\nline2\nline3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := joinSorted(tt.lines)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Note: ApplyCluster requires actual kubectl access, so it's tested in
// integration tests (commands_test.go)
