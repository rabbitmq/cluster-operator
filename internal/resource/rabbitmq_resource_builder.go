package resource

import (
	"strings"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

type ResourceBuilder interface {
	Update(runtime.Object) error
	Build() (runtime.Object, error)
}

func (builder *RabbitmqResourceBuilder) ResourceBuilders() (builders []ResourceBuilder, err error) {
	builders = append(builders, builder.HeadlessService())
	builders = append(builders, builder.IngressService())
	builders = append(builders, builder.ErlangCookie())
	builders = append(builders, builder.AdminSecret())

	return builders, nil
}

func (builder *RabbitmqResourceBuilder) Resources() (resources []runtime.Object, err error) {
	serverConf := builder.ServerConfigMap()
	resources = append(resources, serverConf)

	if builder.DefaultConfiguration.OperatorRegistrySecret != nil {
		clusterRegistrySecret := builder.RegistrySecret()
		resources = append(resources, clusterRegistrySecret)
	}

	serviceAccount := builder.ServiceAccount()
	resources = append(resources, serviceAccount)

	role := builder.Role()
	resources = append(resources, role)

	roleBinding := builder.RoleBinding()
	resources = append(resources, roleBinding)

	return resources, nil
}

func updateLabels(objectMeta *metav1.ObjectMeta, labels map[string]string) {
	if labels != nil {
		if objectMeta.Labels == nil {
			objectMeta.Labels = make(map[string]string)
		}
		for label, value := range labels {
			if !strings.HasPrefix(label, "app.kubernetes.io") {
				// TODO if a label is in the StatefulSet and in the CR, the value in the CR will overwrite the value in STS
				objectMeta.Labels[label] = value
			}
		}
	}
}
