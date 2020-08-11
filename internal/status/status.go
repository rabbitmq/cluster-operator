// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
//

// +kubebuilder:object:generate=true
// +groupName=rabbitmq.com
package status

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AllReplicasReady RabbitmqClusterConditionType = "AllReplicasReady"
	ClusterAvailable RabbitmqClusterConditionType = "ClusterAvailable"
	NoWarnings       RabbitmqClusterConditionType = "NoWarnings"
	ReconcileSuccess RabbitmqClusterConditionType = "ReconcileSuccess"
)

type RabbitmqClusterConditionType string

type RabbitmqClusterCondition struct {
	// Type indicates the scope of RabbitmqCluster status addressed by the condition.
	Type RabbitmqClusterConditionType `json:"type"`
	// True, False, or Unknown
	Status corev1.ConditionStatus `json:"status"`
	// The last time this Condition type changed.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// One word, camel-case reason for current status of the condition.
	Reason string `json:"reason,omitempty"`
	// Full text reason for current status of the condition.
	Message string `json:"message,omitempty"`
}

func newRabbitmqClusterCondition(conditionType RabbitmqClusterConditionType) RabbitmqClusterCondition {
	return RabbitmqClusterCondition{
		Type:               conditionType,
		Status:             corev1.ConditionUnknown,
		LastTransitionTime: metav1.Time{},
	}
}

func (condition *RabbitmqClusterCondition) UpdateState(status corev1.ConditionStatus) {
	if condition.Status != status {
		condition.LastTransitionTime = metav1.Now()
	}
	condition.Status = status
}

func (condition *RabbitmqClusterCondition) UpdateReason(reason string, messages ...string) {
	condition.Reason = reason
	condition.Message = strings.Join(messages, ". ")
}
