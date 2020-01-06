package metadata

import "strings"

func GetAnnotations(instanceAnnotations map[string]string) map[string]string {
	childAnnotations := map[string]string{}
	for key, value := range instanceAnnotations {
		if !strings.Contains(key, "kubernetes.io") && !strings.Contains(key, "k8s.io") {
			childAnnotations[key] = value
		}
	}

	return childAnnotations
}
