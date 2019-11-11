package metadata

type label map[string]string

func Label(instanceName string) label {
	return map[string]string{
		"app.kubernetes.io/name":      instanceName,
		"app.kubernetes.io/component": "rabbitmq",
		"app.kubernetes.io/part-of":   "pivotal-rabbitmq",
	}
}

func LabelSelector(instanceName string) label {
	return map[string]string{
		"app.kubernetes.io/name": instanceName,
	}
}
