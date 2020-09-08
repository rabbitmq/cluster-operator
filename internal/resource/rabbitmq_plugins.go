package resource

import (
	"strings"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/metadata"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var requiredPlugins = []string{
	"rabbitmq_peer_discovery_k8s", // required for clustering
	"rabbitmq_prometheus",         // enforce prometheus metrics
	"rabbitmq_management",
}

const PluginsConfig = "plugins-conf"

type RabbitmqPlugins struct {
	requiredPlugins   []string
	additionalPlugins []string
}

type RabbitmqPluginsConfigMapBuilder struct {
	Instance *rabbitmqv1beta1.RabbitmqCluster
}

func NewRabbitmqPlugins(plugins []rabbitmqv1beta1.Plugin) RabbitmqPlugins {
	additionalPlugins := make([]string, len(plugins))
	for i := range additionalPlugins {
		additionalPlugins[i] = string(plugins[i])
	}

	return RabbitmqPlugins{
		requiredPlugins:   requiredPlugins,
		additionalPlugins: additionalPlugins,
	}
}

func (r *RabbitmqPlugins) DesiredPlugins() []string {
	allPlugins := append(r.requiredPlugins, r.additionalPlugins...)

	check := make(map[string]bool)
	enabledPlugins := make([]string, 0)
	for _, p := range allPlugins {
		if _, ok := check[p]; !ok {
			check[p] = true
			enabledPlugins = append(enabledPlugins, p)
		}
	}
	return enabledPlugins
}

func (r *RabbitmqPlugins) AsString(sep string) string {
	return strings.Join(r.DesiredPlugins(), sep)
}

func (builder *RabbitmqResourceBuilder) RabbitmqPluginsConfigMap() *RabbitmqPluginsConfigMapBuilder {
	return &RabbitmqPluginsConfigMapBuilder{
		Instance: builder.Instance,
	}
}

func (builder *RabbitmqPluginsConfigMapBuilder) UpdateRequiresStsRestart() bool {
	return false
}

func (builder *RabbitmqPluginsConfigMapBuilder) Update(object runtime.Object) error {
	configMap := object.(*corev1.ConfigMap)
	configMap.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)
	configMap.Annotations = metadata.ReconcileAndFilterAnnotations(configMap.GetAnnotations(), builder.Instance.Annotations)

	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}
	configMap.Data["enabled_plugins"] = desiredPluginsAsString(builder.Instance.Spec.Rabbitmq.AdditionalPlugins)
	return nil
}

func (builder *RabbitmqPluginsConfigMapBuilder) Build() (runtime.Object, error) {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.Instance.ChildResourceName(PluginsConfig),
			Namespace: builder.Instance.Namespace,
		},
		Data: map[string]string{
			"enabled_plugins": desiredPluginsAsString([]rabbitmqv1beta1.Plugin{}),
		},
	}, nil
}

func desiredPluginsAsString(additionalPlugins []rabbitmqv1beta1.Plugin) string {
	plugins := NewRabbitmqPlugins(additionalPlugins)
	return "[" + plugins.AsString(",") + "]."
}
