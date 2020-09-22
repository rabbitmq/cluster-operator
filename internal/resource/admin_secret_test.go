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

	"gopkg.in/ini.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/resource"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("AdminSecret", func() {
	var (
		secret             *corev1.Secret
		instance           rabbitmqv1beta1.RabbitmqCluster
		builder            *resource.RabbitmqResourceBuilder
		adminSecretBuilder *resource.AdminSecretBuilder
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
		adminSecretBuilder = builder.AdminSecret()
	})

	Context("Build with defaults", func() {
		It("creates the necessary admin secret", func() {
			var username []byte
			var password []byte
			var ok bool

			obj, err := adminSecretBuilder.Build()
			Expect(err).NotTo(HaveOccurred())
			secret = obj.(*corev1.Secret)

			By("creating the secret with correct name and namespace", func() {
				Expect(secret.Name).To(Equal(instance.ChildResourceName("admin")))
				Expect(secret.Namespace).To(Equal("a namespace"))
			})

			By("creating a 'opaque' secret ", func() {
				Expect(secret.Type).To(Equal(corev1.SecretTypeOpaque))
			})

			By("creating a rabbitmq username that is base64 encoded and 24 characters in length", func() {
				username, ok = secret.Data["username"]
				Expect(ok).NotTo(BeFalse(), "Failed to find a key \"username\" in the generated Secret")
				decodedUsername, err := b64.URLEncoding.DecodeString(string(username))
				Expect(err).NotTo(HaveOccurred())
				Expect(len(decodedUsername)).To(Equal(24))
			})

			By("creating a rabbitmq password that is base64 encoded and 24 characters in length", func() {
				password, ok = secret.Data["password"]
				Expect(ok).NotTo(BeFalse(), "Failed to find a key \"password\" in the generated Secret")
				decodedPassword, err := b64.URLEncoding.DecodeString(string(password))
				Expect(err).NotTo(HaveOccurred())
				Expect(len(decodedPassword)).To(Equal(24))
			})

			By("creating a default_user.conf file that contains the correct sysctl config format to be parsed by RabbitMQ", func() {
				defaultUserConf, ok := secret.Data["default_user.conf"]
				Expect(ok).NotTo(BeFalse(), "Failed to find a key \"default_user.conf\" in the generated Secret")

				cfg, err := ini.Load(defaultUserConf)
				Expect(err).NotTo(HaveOccurred())

				Expect(cfg.Section("").HasKey("default_user")).To(BeTrue())
				Expect(cfg.Section("").HasKey("default_pass")).To(BeTrue())

				Expect(cfg.Section("").Key("default_user").Value()).To(Equal(string(username)))
				Expect(cfg.Section("").Key("default_pass").Value()).To(Equal(string(password)))
			})
		})
	})

	Context("Update with instance labels", func() {
		It("Updates the secret", func() {
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

			By("adding new labels from the CR", func() {
				testLabels(secret.Labels)
			})

			By("restoring the default labels", func() {
				labels := secret.Labels
				Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
				Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
				Expect(labels["app.kubernetes.io/part-of"]).To(Equal("rabbitmq"))
			})

			By("deleting the labels that are removed from the CR", func() {
				Expect(secret.Labels).NotTo(HaveKey("this-was-the-previous-label"))
			})
		})
	})

	Context("Update with instance annotations", func() {
		It("updates the secret with the annotations", func() {
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

			By("updating secret annotations on admin secret", func() {
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
})
