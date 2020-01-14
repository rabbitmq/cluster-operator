package metadata

import "strings"

func ReconcileAnnotations(existing map[string]string, defaults ...map[string]string) map[string]string {
	if len(defaults) == 0 {
		return existing
	}
	return mergeWithFilter(isKubernetesAnnotation, mergeWithFilter(isNotKubernetesAnnotation, map[string]string{}, defaults...), existing)
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
