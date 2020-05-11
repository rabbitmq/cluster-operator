// Copyright (c) 2020 VMware, Inc. or its affiliates.  All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package metadata

import "strings"

func ReconcileAnnotations(existing map[string]string, defaults ...map[string]string) map[string]string {
	if existing == nil {
		existing = map[string]string{}
	}

	if len(defaults) == 0 {
		return existing
	}

	return mergeWithFilter(func(k string) bool { return true }, existing, defaults...)
}

func ReconcileAndFilterAnnotations(existing map[string]string, defaults ...map[string]string) map[string]string {
	if existing == nil {
		existing = map[string]string{}
	}

	if len(defaults) == 0 {
		return existing
	}

	return mergeWithFilter(isNotKubernetesAnnotation, existing, defaults...)
}

func mergeWithFilter(filterFn func(string) bool, base map[string]string, maps ...map[string]string) map[string]string {
	result := base

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
