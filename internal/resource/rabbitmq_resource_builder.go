package resource

import (
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

type RabbitmqResourceBuilder struct {
	Instance *rabbitmqv1beta1.Cluster
	Scheme   *runtime.Scheme
}

type ResourceBuilder interface {
	Update(runtime.Object) error
	Build() (runtime.Object, error)
}

func (builder *RabbitmqResourceBuilder) ResourceBuilders() ([]ResourceBuilder, error) {
	return []ResourceBuilder{
		builder.HeadlessService(),
		builder.IngressService(),
		builder.ErlangCookie(),
		builder.AdminSecret(),
		builder.ServerConfigMap(),
		builder.ServiceAccount(),
		builder.Role(),
		builder.RoleBinding(),
		builder.StatefulSet(),
	}, nil
}
