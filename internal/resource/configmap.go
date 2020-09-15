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

	"gopkg.in/ini.v1"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	serverConfigMapName = "server-conf"
	defaultRabbitmqConf = `
cluster_formation.peer_discovery_backend = rabbit_peer_discovery_k8s
cluster_formation.k8s.host = kubernetes.default
cluster_formation.k8s.address_type = hostname
cluster_formation.node_cleanup.interval = 30
cluster_formation.node_cleanup.only_log_warning = true
cluster_partition_handling = pause_minority
queue_master_locator = min-masters`

	defaultTLSConf = `
ssl_options.certfile = /etc/rabbitmq-tls/tls.crt
ssl_options.keyfile = /etc/rabbitmq-tls/tls.key
listeners.ssl.default = 5671`
)

type ServerConfigMapBuilder struct {
	Instance *rabbitmqv1beta1.RabbitmqCluster
}

func (builder *RabbitmqResourceBuilder) ServerConfigMap() *ServerConfigMapBuilder {
	return &ServerConfigMapBuilder{
		Instance: builder.Instance,
	}
}

func (builder *ServerConfigMapBuilder) UpdateRequiresStsRestart() bool {
	return true // because rabbitmq.conf and advanced.config changes take effect only after a node restart
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
	}

	if builder.Instance.MutualTLSEnabled() {
		if _, err := defaultSection.NewKey("ssl_options.cacertfile", "/etc/rabbitmq-tls/"+builder.Instance.Spec.TLS.CaCertName); err != nil {
			return err
		}
		if _, err := defaultSection.NewKey("ssl_options.verify", "verify_peer"); err != nil {
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

	return nil
}

func (builder *ServerConfigMapBuilder) Build() (runtime.Object, error) {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName(serverConfigMapName),
			Namespace: builder.Instance.Namespace,
		},
	}, nil
}

func updateProperty(configMapData map[string]string, key string, value string) {
	if value == "" {
		delete(configMapData, key)
	} else {
		configMapData[key] = value
	}
}
