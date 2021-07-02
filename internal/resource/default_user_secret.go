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
	AMQPPort              = "5672"
	AMQPSPort             = "5671"
	MQTTPort              = "1883"
	MQTTSPort             = "8883"
	STOMPPort             = "61613"
	STOMPSPort            = "61614"
	streamPort            = "5552"
	streamsPort           = "5551"
	WebMQTTPort           = "15675"
	WebMQTTSPort          = "15676"
	WebSTOMPPort          = "15674"
	WebSTOMPSPort         = "15673"
)

type DefaultUserSecretBuilder struct {
	*RabbitmqResourceBuilder
}

func (builder *RabbitmqResourceBuilder) DefaultUserSecret() *DefaultUserSecretBuilder {
	return &DefaultUserSecretBuilder{builder}
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

	host := fmt.Sprintf("%s.%s.svc.cluster.local", builder.Instance.ChildResourceName("client"), builder.Instance.Namespace)

	// Default user secret implements the service binding Provisioned Service
	// See: https://k8s-service-bindings.github.io/spec/#provisioned-service
	secretData := builder.buildSecretPorts()
	secretData["provider"] = []byte(bindingProvider)
	secretData["type"] = []byte(bindingType)
	secretData["username"] = []byte(username)
	secretData["password"] = []byte(password)
	secretData["host"] = []byte(host)
	secretData["default_user.conf"] = defaultUserConf

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName(DefaultUserSecretName),
			Namespace: builder.Instance.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretData,
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

func (builder *DefaultUserSecretBuilder) buildSecretPorts() map[string][]byte {
	secretData := map[string][]byte{}
	secretData["port"] = []byte(builder.port())

	if builder.pluginEnabled("rabbitmq_mqtt") {
		if builder.Instance.Spec.TLS.SecretName != "" {
			secretData["mqtt-port"] = []byte(MQTTSPort)
		} else {
			secretData["mqtt-port"] = []byte(MQTTPort)
		}
	}

	if builder.pluginEnabled("rabbitmq_stomp") {
		if builder.Instance.Spec.TLS.SecretName != "" {
			secretData["stomp-port"] = []byte(STOMPSPort)
		} else {
			secretData["stomp-port"] = []byte(STOMPPort)
		}
	}

	if builder.pluginEnabled("rabbitmq_stream") {
		if builder.Instance.Spec.TLS.SecretName != "" {
			secretData["stream-port"] = []byte(streamsPort)
		} else {
			secretData["stream-port"] = []byte(streamPort)
		}
	}

	if builder.pluginEnabled("rabbitmq_web_mqtt") {
		if builder.Instance.Spec.TLS.SecretName != "" {
			secretData["web-mqtt-port"] = []byte(WebMQTTSPort)
		} else {
			secretData["web-mqtt-port"] = []byte(WebMQTTPort)
		}
	}

	if builder.pluginEnabled("rabbitmq_web_stomp") {
		if builder.Instance.Spec.TLS.SecretName != "" {
			secretData["web-stomp-port"] = []byte(WebSTOMPSPort)
		} else {
			secretData["web-stomp-port"] = []byte(WebSTOMPPort)
		}
	}

	return secretData
}

func (builder *DefaultUserSecretBuilder) port() string {
	if builder.Instance.Spec.TLS.SecretName != "" {
		return AMQPSPort
	}
	return AMQPPort
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
