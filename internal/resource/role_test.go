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

var _ = Describe("Role", func() {
	var (
		role        *rbacv1.Role
		instance    rabbitmqv1beta1.RabbitmqCluster
		roleBuilder *resource.RoleBuilder
		builder     *resource.RabbitmqResourceBuilder
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
		roleBuilder = builder.Role()
	})

	Context("Build with defaults", func() {
		BeforeEach(func() {
			obj, err := roleBuilder.Build()
			role = obj.(*rbacv1.Role)
			Expect(err).NotTo(HaveOccurred())
		})

		It("generates a correct role", func() {
			Expect(role.Namespace).To(Equal(builder.Instance.Namespace))
			Expect(role.Name).To(Equal(instance.ChildResourceName("endpoint-discovery")))

			Expect(len(role.Rules)).To(Equal(1))

			rule := role.Rules[0]
			Expect(rule.APIGroups).To(Equal([]string{""}))
			Expect(rule.Resources).To(Equal([]string{"endpoints"}))
			Expect(rule.Verbs).To(Equal([]string{"get"}))
		})

		It("only creates the required labels", func() {
			labels := role.Labels
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

			obj, err := roleBuilder.Build()
			role = obj.(*rbacv1.Role)
			Expect(err).NotTo(HaveOccurred())
		})

		It("has the labels from the CRD on the role", func() {
			testLabels(role.Labels)
		})

		It("also has the required labels", func() {
			labels := role.Labels
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

			obj, err := roleBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			role = obj.(*rbacv1.Role)
		})

		It("has the annotations from the CRD on the role", func() {
			expectedAnnotations := map[string]string{"my-annotation": "i-like-this"}

			Expect(role.Annotations).To(Equal(expectedAnnotations))
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

			role = &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":      instance.Name,
						"app.kubernetes.io/part-of":   "pivotal-rabbitmq",
						"this-was-the-previous-label": "should-be-deleted",
					},
				},
			}
			err := roleBuilder.Update(role)
			Expect(err).NotTo(HaveOccurred())
		})

		It("adds labels from the CR", func() {
			testLabels(role.Labels)
		})

		It("restores the default labels", func() {
			labels := role.Labels
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
			Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("pivotal-rabbitmq"))
		})

		It("deletes the labels that are removed from the CR", func() {
			Expect(role.Labels).NotTo(HaveKey("this-was-the-previous-label"))
		})
	})

	Context("Update Rules", func() {
		BeforeEach(func() {
			instance = rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rabbit-labelled",
				},
			}
			role = &rbacv1.Role{
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"foo"},
						Resources: []string{"endpoints", "bar"},
						Verbs:     []string{},
					},
				},
			}

			err := roleBuilder.Update(role)
			Expect(err).NotTo(HaveOccurred())
		})

		It("overwrites the modified rules", func() {
			expectedRules := []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"endpoints"},
					Verbs:     []string{"get"},
				},
			}

			Expect(role.Rules).To(Equal(expectedRules))

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

			role = &rbacv1.Role{
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
			err := roleBuilder.Update(role)
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates role annotations", func() {
			expectedAnnotations := map[string]string{
				"my-annotation":                 "i-like-this",
				"old-annotation":                "old-value",
				"im-here-to-stay.kubernetes.io": "for-a-while",
				"kubernetes.io/name":            "should-stay",
				"kubectl.kubernetes.io/name":    "should-stay",
				"k8s.io/name":                   "should-stay",
			}
			Expect(role.Annotations).To(Equal(expectedAnnotations))
		})
	})
})
