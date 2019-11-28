package resource

import (
	"fmt"
	"strings"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type RabbitmqResourceBuilder struct {
	Instance             *rabbitmqv1beta1.RabbitmqCluster
	DefaultConfiguration DefaultConfiguration
}

type DefaultConfiguration struct {
	ServiceAnnotations         map[string]string
	ServiceType                string
	Scheme                     *runtime.Scheme
	ImageReference             string
	ImagePullSecret            string
	PersistentStorage          string
	PersistentStorageClassName string
	ResourceRequirements       ResourceRequirements
	OperatorRegistrySecret     *corev1.Secret
}

func (cluster *RabbitmqResourceBuilder) Resources() (resources []runtime.Object, err error) {
	serverConf := cluster.ServerConfigMap()
	resources = append(resources, serverConf)

	headlessService := cluster.HeadlessService()
	resources = append(resources, headlessService)

	adminSecret, err := cluster.AdminSecret()
	if err != nil {
		return nil, fmt.Errorf("failed to generate admin secret: %v ", err)
	}
	resources = append(resources, adminSecret)

	erlangCookie, err := cluster.ErlangCookie()
	if err != nil {
		return nil, fmt.Errorf("failed to generate erlang cookie: %v ", err)
	}
	resources = append(resources, erlangCookie)

	if cluster.DefaultConfiguration.OperatorRegistrySecret != nil {
		clusterRegistrySecret := cluster.RegistrySecret()
		resources = append(resources, clusterRegistrySecret)
	}

	serviceAccount := cluster.ServiceAccount()
	resources = append(resources, serviceAccount)

	role := cluster.Role()
	resources = append(resources, role)

	roleBinding := cluster.RoleBinding()
	resources = append(resources, roleBinding)

	return resources, nil
}

func (builder *RabbitmqResourceBuilder) updateLabels(labels map[string]string) map[string]string {
	if builder.Instance.Labels != nil {
		if labels == nil {
			labels = make(map[string]string)
		}
		for label, value := range builder.Instance.Labels {
			if !strings.HasPrefix(label, "app.kubernetes.io") {
				// TODO if a label is in the StatefulSet and in the CR, the value in the CR will overwrite the value in STS
				labels[label] = value
			}
		}
	}

	return labels
}
