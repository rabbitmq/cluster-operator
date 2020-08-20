// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
//

package metadata

import "strings"

func ReconcileAnnotations(existing map[string]string, defaults ...map[string]string) map[string]string {
	return mergeWithFilter(func(k string) bool { return true }, existing, defaults...)
}

func ReconcileAndFilterAnnotations(existing map[string]string, defaults ...map[string]string) map[string]string {
	return mergeWithFilter(isNotKubernetesAnnotation, existing, defaults...)
}

func mergeWithFilter(filterFn func(string) bool, base map[string]string, maps ...map[string]string) map[string]string {
	result := map[string]string{}
	if base != nil {
		result = base
	}

	for _, m := range maps {
		for k, v := range m {
			if filterFn(k) {
				result[k] = v
			}
		}
	}

	return result
}

func isNotKubernetesAnnotation(k string) bool {
	return !isKubernetesAnnotation(k)
}

func isKubernetesAnnotation(k string) bool {
	return strings.Contains(k, "kubernetes.io") || strings.Contains(k, "k8s.io")
}
