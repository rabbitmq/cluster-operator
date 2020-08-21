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
			Expect(err).NotTo(HaveOccurred())
			role = obj.(*rbacv1.Role)
		})

		It("generates correct role metadata", func() {
			Expect(role.Namespace).To(Equal(builder.Instance.Namespace))
			Expect(role.Name).To(Equal(instance.ChildResourceName("peer-discovery")))
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
						"app.kubernetes.io/part-of":   "rabbitmq",
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
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("rabbitmq"))
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
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     []string{"create"},
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
				"my-annotation":               "i-like-this",
				"kubernetes.io/name":          "i-do-not-like-this",
				"kubectl.kubernetes.io/name":  "i-do-not-like-this",
				"k8s.io/name":                 "i-do-not-like-this",
				"kubernetes.io/other":         "i-do-not-like-this",
				"kubectl.kubernetes.io/other": "i-do-not-like-this",
				"k8s.io/other":                "i-do-not-like-this",
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
