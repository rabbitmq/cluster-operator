package resource

import (
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

type RabbitmqResourceBuilder struct {
	Instance *rabbitmqv1beta1.RabbitmqCluster
	Scheme   *runtime.Scheme
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
	builders = append(builders, builder.StatefulSet())
	return builders, nil
}
