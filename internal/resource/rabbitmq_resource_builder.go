package resource

import (
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

type ResourceBuilder interface {
	Update(runtime.Object) error
	Build() (runtime.Object, error)
}

func (builder *RabbitmqResourceBuilder) ResourceBuilders() (builders []ResourceBuilder, err error) {
	builders = append(builders, builder.HeadlessService())
	builders = append(builders, builder.IngressService())
	builders = append(builders, builder.ErlangCookie())
	builders = append(builders, builder.AdminSecret())
	builders = append(builders, builder.ServerConfigMap())
	builders = append(builders, builder.ServiceAccount())
	builders = append(builders, builder.Role())
	builders = append(builders, builder.RoleBinding())

	if builder.DefaultConfiguration.OperatorRegistrySecret != nil {
		builders = append(builders, builder.RegistrySecret())
	}

	builders = append(builders, builder.StatefulSet())
	return builders, nil
}
