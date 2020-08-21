// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
//

package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/resource"
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

	Context("Build", func() {
		BeforeEach(func() {
			obj, err := roleBindingBuilder.Build()
			roleBinding = obj.(*rbacv1.RoleBinding)
			Expect(err).NotTo(HaveOccurred())
		})

		It("generates a correct roleBinding", func() {
			Expect(roleBinding.Namespace).To(Equal(builder.Instance.Namespace))
			Expect(roleBinding.Name).To(Equal(builder.Instance.ChildResourceName("server")))
		})
	})

	Context("Update", func() {
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
						"app.kubernetes.io/part-of":   "rabbitmq",
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
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("rabbitmq"))
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
				Name:     "rabbit-rolebinding-rabbitmq-peer-discovery",
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
