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

	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/rabbitmq/cluster-operator/internal/metadata"
	"gopkg.in/ini.v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rabbitmq/cluster-operator/api/v1beta1"
)

const (
	DefaultUserSecretName = "default-user"
	bindingProvider       = "rabbitmq"
	bindingType           = "rabbitmq"
)

type DefaultUserSecretBuilder struct {
	*RabbitmqResourceBuilder
	secretData map[string][]byte
}

func (builder *RabbitmqResourceBuilder) DefaultUserSecret() *DefaultUserSecretBuilder {
	return &DefaultUserSecretBuilder{builder, map[string][]byte{}}
}

func (builder *DefaultUserSecretBuilder) Build() (client.Object, error) {
	username, err := randomEncodedString(24)
	if err != nil {
		return nil, err
	}

	password, err := randomEncodedString(24)
	if err != nil {
		return nil, err
	}

	defaultUserConf, err := generateDefaultUserConf(username, password)
	if err != nil {
		return nil, err
	}

	host := fmt.Sprintf("%s.%s.svc.cluster.local", builder.Instance.Name, builder.Instance.Namespace)

	// Default user secret implements the service binding Provisioned Service
	// See: https://k8s-service-bindings.github.io/spec/#provisioned-service
	builder.secretData["username"] = []byte(username)
	builder.secretData["password"] = []byte(password)
	builder.secretData["default_user.conf"] = defaultUserConf
	builder.secretData["provider"] = []byte(bindingProvider)
	builder.secretData["type"] = []byte(bindingType)
	builder.secretData["host"] = []byte(host)
	builder.addPortsToSecret()

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName(DefaultUserSecretName),
			Namespace: builder.Instance.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: builder.secretData,
	}, nil
}

func (builder *DefaultUserSecretBuilder) UpdateMayRequireStsRecreate() bool {
	return false
}

func (builder *DefaultUserSecretBuilder) Update(object client.Object) error {
	secret := object.(*corev1.Secret)
	secret.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)
	secret.Annotations = metadata.ReconcileAndFilterAnnotations(secret.GetAnnotations(), builder.Instance.Annotations)

	if err := controllerutil.SetControllerReference(builder.Instance, secret, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %v", err)
	}

	return nil
}

func (builder *DefaultUserSecretBuilder) addPortsToSecret() {
	const (
		AMQPPort  = "5672"
		AMQPSPort = "5671"
	)
	portNames := map[v1beta1.Plugin]string{
		"rabbitmq_mqtt":      "mqtt-port",
		"rabbitmq_stomp":     "stomp-port",
		"rabbitmq_stream":    "stream-port",
		"rabbitmq_web_mqtt":  "web-mqtt-port",
		"rabbitmq_web_stomp": "web-stomp-port",
	}
	TLSPort := map[string]string{
		"mqtt-port":      "8883",
		"stomp-port":     "61614",
		"stream-port":    "5551",
		"web-mqtt-port":  "15676",
		"web-stomp-port": "15673",
	}
	port := map[string]string{
		"mqtt-port":      "1883",
		"stomp-port":     "61613",
		"stream-port":    "5552",
		"web-mqtt-port":  "15675",
		"web-stomp-port": "15674",
	}

	if builder.Instance.Spec.TLS.SecretName != "" {
		builder.secretData["port"] = []byte(AMQPSPort)

		for plugin, portName := range portNames {
			if builder.pluginEnabled(plugin) {
				builder.secretData[portName] = []byte(TLSPort[portName])
			}
		}
	} else {
		builder.secretData["port"] = []byte(AMQPPort)

		for plugin, portName := range portNames {
			if builder.pluginEnabled(plugin) {
				builder.secretData[portName] = []byte(port[portName])
			}
		}
	}
}

func (builder *DefaultUserSecretBuilder) pluginEnabled(plugin v1beta1.Plugin) bool {
	for _, value := range builder.Instance.Spec.Rabbitmq.AdditionalPlugins {
		if value == plugin {
			return true
		}
	}
	return false
}

func generateDefaultUserConf(username, password string) ([]byte, error) {
	ini.PrettySection = false // Remove trailing new line because default_user.conf has only a default section.
	cfg, err := ini.Load([]byte{})
	if err != nil {
		return nil, err
	}
	defaultSection := cfg.Section("")

	if _, err := defaultSection.NewKey("default_user", username); err != nil {
		return nil, err
	}

	if _, err := defaultSection.NewKey("default_pass", password); err != nil {
		return nil, err
	}

	var userConfBuffer bytes.Buffer
	if _, err := cfg.WriteTo(&userConfBuffer); err != nil {
		return nil, err
	}

	return userConfBuffer.Bytes(), nil
}
