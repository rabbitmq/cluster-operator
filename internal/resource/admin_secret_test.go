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
	b64 "encoding/base64"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("AdminSecret", func() {
	var (
		secret             *corev1.Secret
		instance           rabbitmqv1beta1.RabbitmqCluster
		rabbitmqCluster    *resource.RabbitmqResourceBuilder
		adminSecretBuilder *resource.AdminSecretBuilder
	)

	BeforeEach(func() {
		instance = rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: v1.ObjectMeta{
				Name:      "a name",
				Namespace: "a namespace",
			},
		}
		rabbitmqCluster = &resource.RabbitmqResourceBuilder{
			Instance: &instance,
		}
		adminSecretBuilder = rabbitmqCluster.AdminSecret()
	})

	Context("Build with defaults", func() {
		BeforeEach(func() {
			obj, err := adminSecretBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			secret = obj.(*corev1.Secret)
		})

		It("creates the secret with correct name and namespace", func() {
			Expect(secret.Name).To(Equal(instance.ChildResourceName("admin")))
			Expect(secret.Namespace).To(Equal("a namespace"))
		})

		It("creates a 'opaque' secret ", func() {
			Expect(secret.Type).To(Equal(corev1.SecretTypeOpaque))
		})

		It("creates a rabbitmq username that is base64 encoded and 24 characters in length", func() {
			username, ok := secret.Data["username"]
			Expect(ok).NotTo(BeFalse())
			decodedUsername, err := b64.URLEncoding.DecodeString(string(username))
			Expect(err).NotTo(HaveOccurred())
			Expect(len(decodedUsername)).To(Equal(24))

		})

		It("creates a rabbitmq password that is base64 encoded and 24 characters in length", func() {
			password, ok := secret.Data["password"]
			Expect(ok).NotTo(BeFalse())
			decodedPassword, err := b64.URLEncoding.DecodeString(string(password))
			Expect(err).NotTo(HaveOccurred())
			Expect(len(decodedPassword)).To(Equal(24))
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

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":      instance.Name,
						"app.kubernetes.io/part-of":   "rabbitmq",
						"this-was-the-previous-label": "should-be-deleted",
					},
				},
			}
			err := adminSecretBuilder.Update(secret)
			Expect(err).NotTo(HaveOccurred())
		})

		It("adds new labels from the CR", func() {
			testLabels(secret.Labels)
		})

		It("restores the default labels", func() {
			labels := secret.Labels
			Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
			Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
			Expect(labels["app.kubernetes.io/part-of"]).To(Equal("rabbitmq"))
		})

		It("deletes the labels that are removed from the CR", func() {
			Expect(secret.Labels).NotTo(HaveKey("this-was-the-previous-label"))
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

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"i-was-here-already":            "please-dont-delete-me",
						"im-here-to-stay.kubernetes.io": "for-a-while",
						"kubernetes.io/name":            "should-stay",
						"kubectl.kubernetes.io/name":    "should-stay",
						"k8s.io/name":                   "should-stay",
					},
				},
			}
			err := adminSecretBuilder.Update(secret)
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates secret annotations on admin secret", func() {
			expectedAnnotations := map[string]string{
				"my-annotation":                 "i-like-this",
				"i-was-here-already":            "please-dont-delete-me",
				"im-here-to-stay.kubernetes.io": "for-a-while",
				"kubernetes.io/name":            "should-stay",
				"kubectl.kubernetes.io/name":    "should-stay",
				"k8s.io/name":                   "should-stay",
			}

			Expect(secret.Annotations).To(Equal(expectedAnnotations))
		})
	})
})
