// Copyright (c) 2020 VMware, Inc. or its affiliates.  All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package status

import (
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func AllReplicasReadyCondition(resources []runtime.Object,
	existingCondition *RabbitmqClusterCondition) RabbitmqClusterCondition {

	condition := generateCondition(AllReplicasReady)
	if existingCondition != nil {
		condition.LastTransitionTime = existingCondition.LastTransitionTime
	}

	for index := range resources {
		switch resource := resources[index].(type) {
		case *appsv1.StatefulSet:
			if resource == nil {
				condition.Status = corev1.ConditionUnknown
				condition.Reason = "MissingStatefulSet"
				condition.Message = "Could not find StatefulSet"
				goto assignLastTransitionTime
			}

			if resource.Status.Replicas == resource.Status.ReadyReplicas {
				condition.Status = corev1.ConditionTrue
				condition.Reason = "AllPodsAreReady"
				goto assignLastTransitionTime
			}

			condition.Status = corev1.ConditionFalse
			condition.Reason = "NotAllPodsReady"
			condition.Message = fmt.Sprintf("%d/%d Pods ready",
				resource.Status.ReadyReplicas,
				resource.Status.Replicas)
		}
	}

assignLastTransitionTime:
	if existingCondition == nil || existingCondition.Status != condition.Status {
		condition.LastTransitionTime = metav1.Time{
			Time: time.Now(),
		}
	}

	return condition
}
