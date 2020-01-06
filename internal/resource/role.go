package resource

import (
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/metadata"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	roleName = "endpoint-discovery"
)

type RoleBuilder struct {
	Instance             *rabbitmqv1beta1.RabbitmqCluster
	DefaultConfiguration DefaultConfiguration
}

func (builder *RabbitmqResourceBuilder) Role() *RoleBuilder {
	return &RoleBuilder{
		Instance:             builder.Instance,
		DefaultConfiguration: builder.DefaultConfiguration,
	}
}

func (builder *RoleBuilder) Update(object runtime.Object) error {
	role := object.(*rbacv1.Role)
	role.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)
	role.Annotations = metadata.GetAnnotations(builder.Instance.Annotations)
	return nil
}

func (builder *RoleBuilder) Build() (runtime.Object, error) {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   builder.Instance.Namespace,
			Name:        builder.Instance.ChildResourceName(roleName),
			Labels:      metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels),
			Annotations: metadata.GetAnnotations(builder.Instance.Annotations),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"endpoints"},
				Verbs:     []string{"get"},
			},
		},
	}, nil
}
