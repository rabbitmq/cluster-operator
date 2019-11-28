package resource

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/rabbitmq-for-kubernetes/internal/metadata"
)

const (
	serviceAccountName = "server"
	roleName           = "endpoint-discovery"
	roleBindingName    = "server"
)

func (builder *RabbitmqResourceBuilder) ServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: builder.Instance.Namespace,
			Name:      builder.Instance.ChildResourceName(serviceAccountName),
			Labels:    metadata.Label(builder.Instance.Name),
		},
	}
}

func (builder *RabbitmqResourceBuilder) Role() *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: builder.Instance.Namespace,
			Name:      builder.Instance.ChildResourceName(roleName),
			Labels:    metadata.Label(builder.Instance.Name),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"endpoints"},
				Verbs:     []string{"get"},
			},
		},
	}
}

func (builder *RabbitmqResourceBuilder) RoleBinding() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: builder.Instance.Namespace,
			Name:      builder.Instance.ChildResourceName(roleBindingName),
			Labels:    metadata.Label(builder.Instance.Name),
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
	}
}
