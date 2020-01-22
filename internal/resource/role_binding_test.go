package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("RoleBinding", func() {
	var (
		roleBinding        *rbacv1.RoleBinding
		instance           rabbitmqv1beta1.RabbitmqCluster
		roleBindingBuilder *resource.RoleBindingBuilder
		builder            *resource.RabbitmqResourceBuilder
	)

	BeforeEach(func() {
		instance = rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: v1.ObjectMeta{
				Name:      "a name",
				Namespace: "a namespace",
			},
		}
		builder = &resource.RabbitmqResourceBuilder{
			Instance: &instance,
		}
		roleBindingBuilder = builder.RoleBinding()
	})

	Context("Build with defaults", func() {
		BeforeEach(func() {
			obj, err := roleBindingBuilder.Build()
			roleBinding = obj.(*rbacv1.RoleBinding)
			Expect(err).NotTo(HaveOccurred())
		})

		It("generates a correct roleBinding", func() {
			Expect(roleBinding.Namespace).To(Equal(builder.Instance.Namespace))
			Expect(roleBinding.Name).To(Equal(builder.Instance.ChildResourceName("server")))

			Expect(len(roleBinding.Subjects)).To(Equal(1))
			subject := roleBinding.Subjects[0]

			Expect(subject.Kind).To(Equal("ServiceAccount"))
			Expect(subject.Name).To(Equal(builder.Instance.ChildResourceName("server")))

			Expect(roleBinding.RoleRef.APIGroup).To(Equal("rbac.authorization.k8s.io"))
			Expect(roleBinding.RoleRef.Kind).To(Equal("Role"))
			Expect(roleBinding.RoleRef.Name).To(Equal(builder.Instance.ChildResourceName("endpoint-discovery")))
		})

		It("only creates the required labels", func() {
			labels := roleBinding.Labels
			Expect(len(labels)).To(Equal(3))
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
			Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
		})
	})

	Context("Build with instance labels", func() {
		BeforeEach(func() {
			instance.Labels = map[string]string{
				"app.kubernetes.io/foo": "bar",
				"foo":                   "bar",
				"rabbitmq":              "is-great",
				"foo/app.kubernetes.io": "edgecase",
			}

			obj, err := roleBindingBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			roleBinding = obj.(*rbacv1.RoleBinding)
		})

		It("has the labels from the CRD on the roleBinding", func() {
			testLabels(roleBinding.Labels)
		})

		It("also has the required labels", func() {
			labels := roleBinding.Labels
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
			Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
		})
	})

	Context("Build with instance annotations", func() {
		BeforeEach(func() {
			instance.Annotations = map[string]string{
				"my-annotation":              "i-like-this",
				"kubernetes.io/name":         "i-do-not-like-this",
				"kubectl.kubernetes.io/name": "i-do-not-like-this",
				"k8s.io/name":                "i-do-not-like-this",
			}

			obj, err := roleBindingBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			roleBinding = obj.(*rbacv1.RoleBinding)
		})

		It("has the annotations from the CRD on the role binding", func() {
			expectedAnnotations := map[string]string{"my-annotation": "i-like-this"}
			Expect(roleBinding.Annotations).To(Equal(expectedAnnotations))
		})
	})

	Context("Update with instance labels", func() {
		BeforeEach(func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rabbit-labelled",
				},
			}
			instance.Labels = map[string]string{
				"app.kubernetes.io/foo": "bar",
				"foo":                   "bar",
				"rabbitmq":              "is-great",
				"foo/app.kubernetes.io": "edgecase",
			}

			roleBinding = &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":      instance.Name,
						"app.kubernetes.io/part-of":   "pivotal-rabbitmq",
						"this-was-the-previous-label": "should-be-deleted",
					},
				},
			}
			err := roleBindingBuilder.Update(roleBinding)
			Expect(err).NotTo(HaveOccurred())
		})

		It("adds labels from the CR", func() {
			testLabels(roleBinding.Labels)
		})

		It("restores the default labels", func() {
			labels := roleBinding.Labels
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
			Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
		})

		It("deletes the labels that are removed from the CR", func() {
			Expect(roleBinding.Labels).NotTo(HaveKey("this-was-the-previous-label"))
		})
	})

	Context("Update with required rules", func() {
		BeforeEach(func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rabbit-rolebinding",
				},
			}
			builder.Instance = &instance
			roleBinding = &rbacv1.RoleBinding{
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "RoleRoleRole",
					Name:     "NameNameName",
				},
				Subjects: []rbacv1.Subject{
					{
						Kind: "AccountService",
						Name: "this-account-is-not-right",
					},
				},
			}

			err := roleBindingBuilder.Update(roleBinding)
			Expect(err).NotTo(HaveOccurred())
		})

		It("sets the required role ref and subjects", func() {
			expectedRoleRef := rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "rabbit-rolebinding-rabbitmq-endpoint-discovery",
			}
			expectedSubjects := []rbacv1.Subject{
				{
					Kind: "ServiceAccount",
					Name: "rabbit-rolebinding-rabbitmq-server",
				},
			}

			Expect(roleBinding.RoleRef).To(Equal(expectedRoleRef))
			Expect(roleBinding.Subjects).To(Equal(expectedSubjects))
		})
	})

	Context("Update with instance annotations", func() {
		BeforeEach(func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rabbit-labelled",
				},
			}
			instance.Annotations = map[string]string{
				"my-annotation":              "i-like-this",
				"kubernetes.io/name":         "i-do-not-like-this",
				"kubectl.kubernetes.io/name": "i-do-not-like-this",
				"k8s.io/name":                "i-do-not-like-this",
			}

			roleBinding = &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"old-annotation":                "old-value",
						"im-here-to-stay.kubernetes.io": "for-a-while",
						"kubernetes.io/name":            "should-stay",
						"kubectl.kubernetes.io/name":    "should-stay",
						"k8s.io/name":                   "should-stay",
					},
				},
			}
			err := roleBindingBuilder.Update(roleBinding)
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates roleBinding annotations", func() {
			expectedAnnotations := map[string]string{
				"my-annotation":                 "i-like-this",
				"old-annotation":                "old-value",
				"im-here-to-stay.kubernetes.io": "for-a-while",
				"kubernetes.io/name":            "should-stay",
				"kubectl.kubernetes.io/name":    "should-stay",
				"k8s.io/name":                   "should-stay",
			}
			Expect(roleBinding.Annotations).To(Equal(expectedAnnotations))
		})
	})
})
