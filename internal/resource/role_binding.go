package resource

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/metadata"
)

const (
	roleBindingName = "server"
)

type RoleBindingBuilder struct {
	Instance *rabbitmqv1beta1.RabbitmqCluster
}

func (builder *RabbitmqResourceBuilder) RoleBinding() *RoleBindingBuilder {
	return &RoleBindingBuilder{
		Instance: builder.Instance,
	}
}

func (builder *RoleBindingBuilder) Update(object runtime.Object) error {
	roleBinding := object.(*rbacv1.RoleBinding)
	roleBinding.Labels = metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels)
	roleBinding.Annotations = metadata.ReconcileAnnotations(roleBinding.GetAnnotations(), builder.Instance.Annotations)
	return nil
}

func (builder *RoleBindingBuilder) Build() (runtime.Object, error) {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   builder.Instance.Namespace,
			Name:        builder.Instance.ChildResourceName(roleBindingName),
			Labels:      metadata.GetLabels(builder.Instance.Name, builder.Instance.Labels),
			Annotations: metadata.ReconcileAnnotations(map[string]string{}, builder.Instance.Annotations),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     builder.Instance.ChildResourceName(roleName),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: builder.Instance.ChildResourceName(serviceAccountName),
			},
		},
	}, nil
}
