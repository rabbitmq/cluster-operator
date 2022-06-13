// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
//

package resource

import (
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"gopkg.in/ini.v1"

	"github.com/rabbitmq/cluster-operator/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ServerConfigMapName = "server-conf"
	defaultRabbitmqConf = `
cluster_formation.peer_discovery_backend = rabbit_peer_discovery_k8s
cluster_formation.k8s.host = kubernetes.default
cluster_formation.k8s.address_type = hostname
cluster_partition_handling = pause_minority
queue_master_locator = min-masters
disk_free_limit.absolute = 2GB
cluster_formation.randomized_startup_delay_range.min = 0
cluster_formation.randomized_startup_delay_range.max = 60`

	defaultTLSConf = `
ssl_options.certfile = /etc/rabbitmq-tls/tls.crt
ssl_options.keyfile = /etc/rabbitmq-tls/tls.key
listeners.ssl.default = 5671

management.ssl.certfile   = /etc/rabbitmq-tls/tls.crt
management.ssl.keyfile    = /etc/rabbitmq-tls/tls.key
management.ssl.port       = 15671

prometheus.ssl.certfile  = /etc/rabbitmq-tls/tls.crt
prometheus.ssl.keyfile   = /etc/rabbitmq-tls/tls.key
prometheus.ssl.port      = 15691
`
	tlsCertDir      = "/etc/rabbitmq-tls/"
	caCertFilename  = "ca.crt"
	caCertPath      = tlsCertDir + caCertFilename
	tlsCertFilename = "tls.crt"
	tlsCertPath     = tlsCertDir + tlsCertFilename
	tlsKeyFilename  = "tls.key"
	tlsKeyPath      = tlsCertDir + tlsKeyFilename
)

type ServerConfigMapBuilder struct {
	*RabbitmqResourceBuilder
}

func (builder *RabbitmqResourceBuilder) ServerConfigMap() *ServerConfigMapBuilder {
	return &ServerConfigMapBuilder{builder}
}

func (builder *ServerConfigMapBuilder) Build() (client.Object, error) {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        builder.Instance.ChildResourceName(ServerConfigMapName),
			Namespace:   builder.Instance.Namespace,
			Labels:      metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels),
			Annotations: metadata.ReconcileAndFilterAnnotations(nil, builder.Instance.Annotations),
		},
	}, nil
}

func (builder *ServerConfigMapBuilder) UpdateMayRequireStsRecreate() bool {
	return false
}

func (builder *ServerConfigMapBuilder) Update(object client.Object) error {
	configMap := object.(*corev1.ConfigMap)

	ini.PrettySection = false // Remove trailing new line because rabbitmq.conf has only a default section.
	operatorConfiguration, err := ini.Load([]byte(defaultRabbitmqConf))
	if err != nil {
		return err
	}
	defaultSection := operatorConfiguration.Section("")

	if _, err := defaultSection.NewKey("cluster_name", builder.Instance.Name); err != nil {
		return err
	}

	userConfiguration := ini.Empty(ini.LoadOptions{})
	userConfigurationSection := userConfiguration.Section("")

	if builder.Instance.TLSEnabled() {
		if err := userConfiguration.Append([]byte(defaultTLSConf)); err != nil {
			return err
		}
		if builder.Instance.DisableNonTLSListeners() {
			if _, err := userConfigurationSection.NewKey("listeners.tcp", "none"); err != nil {
				return err
			}
		} else {
			// management plugin does not have a *.listeners.tcp settings like other plugins
			// management tcp listener can be disabled by setting management.ssl.port without setting management.tcp.port
			// we set management tcp listener only if tls is enabled and disableNonTLSListeners is false
			if _, err := userConfigurationSection.NewKey("management.tcp.port", "15672"); err != nil {
				return err
			}

			if _, err := userConfigurationSection.NewKey("prometheus.tcp.port", "15692"); err != nil {
				return err
			}
		}
		if builder.Instance.AdditionalPluginEnabled("rabbitmq_mqtt") {
			if _, err := userConfigurationSection.NewKey("mqtt.listeners.ssl.default", "8883"); err != nil {
				return err
			}
			if builder.Instance.DisableNonTLSListeners() {
				if _, err := userConfigurationSection.NewKey("mqtt.listeners.tcp", "none"); err != nil {
					return err
				}
			}
		}
		if builder.Instance.AdditionalPluginEnabled("rabbitmq_stomp") {
			if _, err := userConfigurationSection.NewKey("stomp.listeners.ssl.1", "61614"); err != nil {
				return err
			}
			if builder.Instance.DisableNonTLSListeners() {
				if _, err := userConfigurationSection.NewKey("stomp.listeners.tcp", "none"); err != nil {
					return err
				}
			}
		}
		if builder.Instance.AdditionalPluginEnabled("rabbitmq_stream") {
			if _, err := userConfigurationSection.NewKey("stream.listeners.ssl.default", "5551"); err != nil {
				return err
			}
			if builder.Instance.DisableNonTLSListeners() {
				if _, err := userConfigurationSection.NewKey("stream.listeners.tcp", "none"); err != nil {
					return err
				}
			}
		}
	}

	if builder.Instance.MutualTLSEnabled() {
		if _, err := userConfigurationSection.NewKey("ssl_options.cacertfile", caCertPath); err != nil {
			return err
		}
		if _, err := userConfigurationSection.NewKey("ssl_options.verify", "verify_peer"); err != nil {
			return err
		}

		if _, err := userConfigurationSection.NewKey("management.ssl.cacertfile", caCertPath); err != nil {
			return err
		}

		if _, err := userConfigurationSection.NewKey("prometheus.ssl.cacertfile", caCertPath); err != nil {
			return err
		}

		if builder.Instance.AdditionalPluginEnabled("rabbitmq_web_mqtt") {
			if _, err := userConfigurationSection.NewKey("web_mqtt.ssl.port", "15676"); err != nil {
				return err
			}
			if _, err := userConfigurationSection.NewKey("web_mqtt.ssl.cacertfile", caCertPath); err != nil {
				return err
			}
			if _, err := userConfigurationSection.NewKey("web_mqtt.ssl.certfile", tlsCertPath); err != nil {
				return err
			}
			if _, err := userConfigurationSection.NewKey("web_mqtt.ssl.keyfile", tlsKeyPath); err != nil {
				return err
			}
			if builder.Instance.DisableNonTLSListeners() {
				if _, err := userConfigurationSection.NewKey("web_mqtt.tcp.listener", "none"); err != nil {
					return err
				}
			}
		}
		if builder.Instance.AdditionalPluginEnabled("rabbitmq_web_stomp") {
			if _, err := userConfigurationSection.NewKey("web_stomp.ssl.port", "15673"); err != nil {
				return err
			}
			if _, err := userConfigurationSection.NewKey("web_stomp.ssl.cacertfile", caCertPath); err != nil {
				return err
			}
			if _, err := userConfigurationSection.NewKey("web_stomp.ssl.certfile", tlsCertPath); err != nil {
				return err
			}
			if _, err := userConfigurationSection.NewKey("web_stomp.ssl.keyfile", tlsKeyPath); err != nil {
				return err
			}
			if builder.Instance.DisableNonTLSListeners() {
				if _, err := userConfigurationSection.NewKey("web_stomp.tcp.listener", "none"); err != nil {
					return err
				}
			}
		}
	}

	if builder.Instance.MemoryLimited() {
		if _, err := userConfigurationSection.NewKey("total_memory_available_override_value", fmt.Sprintf("%d", removeHeadroom(builder.Instance.Spec.Resources.Limits.Memory().Value()))); err != nil {
			return err
		}
	}

	var rmqConfBuffer strings.Builder
	if _, err := operatorConfiguration.WriteTo(&rmqConfBuffer); err != nil {
		return err
	}

	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}

	configMap.Data["operatorDefaults.conf"] = rmqConfBuffer.String()

	rmqConfBuffer.Reset()

	rmqProperties := builder.Instance.Spec.Rabbitmq
	if err := userConfiguration.Append([]byte(rmqProperties.AdditionalConfig)); err != nil {
		return fmt.Errorf("failed to append spec.rabbitmq.additionalConfig: %w", err)
	}

	if _, err := userConfiguration.WriteTo(&rmqConfBuffer); err != nil {
		return err
	}

	configMap.Data["userDefinedConfiguration.conf"] = rmqConfBuffer.String()

	updateProperty(configMap.Data, "advanced.config", rmqProperties.AdvancedConfig)
	updateProperty(configMap.Data, "rabbitmq-env.conf", rmqProperties.EnvConfig)

	if err := controllerutil.SetControllerReference(builder.Instance, configMap, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %w", err)
	}

	return nil
}

func updateProperty(configMapData map[string]string, key string, value string) {
	if value == "" {
		delete(configMapData, key)
	} else {
		configMapData[key] = value
	}
}

// The Erlang VM needs headroom above Rabbit to avoid being OOM killed
// We set the headroom to be the smaller amount of 20% memory or 2GiB
func removeHeadroom(memLimit int64) int64 {
	const GiB int64 = 1073741824
	if memLimit/5 > 2*GiB {
		return memLimit - 2*GiB
	}
	return memLimit - memLimit/5
}
