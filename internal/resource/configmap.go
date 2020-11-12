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
	"bytes"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"gopkg.in/ini.v1"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	ServerConfigMapName = "server-conf"
	defaultRabbitmqConf = `
cluster_formation.peer_discovery_backend = rabbit_peer_discovery_k8s
cluster_formation.k8s.host = kubernetes.default
cluster_formation.k8s.address_type = hostname
cluster_partition_handling = pause_minority
queue_master_locator = min-masters
disk_free_limit.absolute = 2GB`

	defaultTLSConf = `
ssl_options.certfile = /etc/rabbitmq-tls/tls.crt
ssl_options.keyfile = /etc/rabbitmq-tls/tls.key
listeners.ssl.default = 5671

management.ssl.certfile   = /etc/rabbitmq-tls/tls.crt
management.ssl.keyfile    = /etc/rabbitmq-tls/tls.key
management.ssl.port       = 15671
`
)

type ServerConfigMapBuilder struct {
	Instance *rabbitmqv1beta1.RabbitmqCluster
	Scheme   *runtime.Scheme
}

func (builder *RabbitmqResourceBuilder) ServerConfigMap() *ServerConfigMapBuilder {
	return &ServerConfigMapBuilder{
		Instance: builder.Instance,
		Scheme:   builder.Scheme,
	}
}

func (builder *ServerConfigMapBuilder) Build() (runtime.Object, error) {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName(ServerConfigMapName),
			Namespace: builder.Instance.Namespace,
		},
	}, nil
}

func (builder *ServerConfigMapBuilder) Update(object runtime.Object) error {
	configMap := object.(*corev1.ConfigMap)
	configMap.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)
	configMap.Annotations = metadata.ReconcileAndFilterAnnotations(configMap.GetAnnotations(), builder.Instance.Annotations)

	ini.PrettySection = false // Remove trailing new line because rabbitmq.conf has only a default section.
	cfg, err := ini.Load([]byte(defaultRabbitmqConf))
	if err != nil {
		return err
	}
	defaultSection := cfg.Section("")

	if _, err := defaultSection.NewKey("cluster_name", builder.Instance.Name); err != nil {
		return err
	}

	if builder.Instance.TLSEnabled() {
		if err := cfg.Append([]byte(defaultTLSConf)); err != nil {
			return err
		}
		if builder.Instance.AdditionalPluginEnabled("rabbitmq_mqtt") {
			if _, err := defaultSection.NewKey("mqtt.listeners.ssl.default", "8883"); err != nil {
				return err
			}
		}
		if builder.Instance.AdditionalPluginEnabled("rabbitmq_stomp") {
			if _, err := defaultSection.NewKey("stomp.listeners.ssl.1", "61614"); err != nil {
				return err
			}
		}
	}

	if builder.Instance.MutualTLSEnabled() {
		if _, err := defaultSection.NewKey("ssl_options.cacertfile", "/etc/rabbitmq-tls/ca.crt"); err != nil {
			return err
		}
		if _, err := defaultSection.NewKey("ssl_options.verify", "verify_peer"); err != nil {
			return err
		}

		if _, err := defaultSection.NewKey("management.ssl.cacertfile", "/etc/rabbitmq-tls/ca.crt"); err != nil {
			return err
		}
		if builder.Instance.AdditionalPluginEnabled("rabbitmq_web_mqtt") {
			if _, err := defaultSection.NewKey("web_mqtt.ssl.port", "15676"); err != nil {
				return err
			}
			if _, err := defaultSection.NewKey("web_mqtt.ssl.cacertfile", "/etc/rabbitmq-tls/ca.crt"); err != nil {
				return err
			}
			if _, err := defaultSection.NewKey("web_mqtt.ssl.certfile", "/etc/rabbitmq-tls/tls.crt"); err != nil {
				return err
			}
			if _, err := defaultSection.NewKey("web_mqtt.ssl.keyfile", "/etc/rabbitmq-tls/tls.key"); err != nil {
				return err
			}
		}
		if builder.Instance.AdditionalPluginEnabled("rabbitmq_web_stomp") {
			if _, err := defaultSection.NewKey("web_stomp.ssl.port", "15673"); err != nil {
				return err
			}
			if _, err := defaultSection.NewKey("web_stomp.ssl.cacertfile", "/etc/rabbitmq-tls/ca.crt"); err != nil {
				return err
			}
			if _, err := defaultSection.NewKey("web_stomp.ssl.certfile", "/etc/rabbitmq-tls/tls.crt"); err != nil {
				return err
			}
			if _, err := defaultSection.NewKey("web_stomp.ssl.keyfile", "/etc/rabbitmq-tls/tls.key"); err != nil {
				return err
			}
		}
	}

	if builder.Instance.MemoryLimited() {
		if _, err := defaultSection.NewKey("total_memory_available_override_value", fmt.Sprintf("%d", removeHeadroom(builder.Instance.Spec.Resources.Limits.Memory().Value()))); err != nil {
			return err
		}
	}

	rmqProperties := builder.Instance.Spec.Rabbitmq
	if err := cfg.Append([]byte(rmqProperties.AdditionalConfig)); err != nil {
		return fmt.Errorf("failed to append spec.rabbitmq.additionalConfig: %w", err)
	}

	var rmqConfBuffer bytes.Buffer
	if _, err := cfg.WriteTo(&rmqConfBuffer); err != nil {
		return err
	}

	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}

	configMap.Data["rabbitmq.conf"] = rmqConfBuffer.String()

	updateProperty(configMap.Data, "advanced.config", rmqProperties.AdvancedConfig)
	updateProperty(configMap.Data, "rabbitmq-env.conf", rmqProperties.EnvConfig)

	if err := controllerutil.SetControllerReference(builder.Instance, configMap, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %v", err)
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
