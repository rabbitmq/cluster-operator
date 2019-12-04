package resource

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/rabbitmq-for-kubernetes/internal/metadata"
)

const (
	roleBindingName = "server"
)

// func (builder *RabbitmqResourceBuilder) Role() *rbacv1.Role {
// 	role := &rbacv1.Role{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Namespace: builder.Instance.Namespace,
// 			Name:      builder.Instance.ChildResourceName(roleName),
// 			Labels:    metadata.Label(builder.Instance.Name),
// 		},
// 		Rules: []rbacv1.PolicyRule{
// 			{
// 				APIGroups: []string{""},
// 				Resources: []string{"endpoints"},
// 				Verbs:     []string{"get"},
// 			},
// 		},
// 	}
// 	updateLabels(&role.ObjectMeta, builder.Instance.Labels)
// 	return role
// }

func (builder *RabbitmqResourceBuilder) RoleBinding() *rbacv1.RoleBinding {
	rolebinding := &rbacv1.RoleBinding{
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
	updateLabels(&rolebinding.ObjectMeta, builder.Instance.Labels)
	return rolebinding
}
