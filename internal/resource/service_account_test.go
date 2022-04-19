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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("ServiceAccount", func() {
	var (
		serviceAccount        *corev1.ServiceAccount
		instance              rabbitmqv1beta1.RabbitmqCluster
		serviceAccountBuilder *resource.ServiceAccountBuilder
		builder               *resource.RabbitmqResourceBuilder
		scheme                *runtime.Scheme
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
		Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())
		instance = rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: v1.ObjectMeta{
				Name:      "a name",
				Namespace: "a namespace",
			},
		}
		builder = &resource.RabbitmqResourceBuilder{
			Instance: &instance,
			Scheme:   scheme,
		}
		serviceAccountBuilder = builder.ServiceAccount()
	})

	Context("Build", func() {
		BeforeEach(func() {
			obj, err := serviceAccountBuilder.Build()
			serviceAccount = obj.(*corev1.ServiceAccount)
			Expect(err).NotTo(HaveOccurred())
		})

		It("generates a ServiceAccount with the correct name and namespace", func() {
			Expect(serviceAccount.Name).To(Equal(builder.Instance.ChildResourceName("server")))
			Expect(serviceAccount.Namespace).To(Equal(builder.Instance.Namespace))
		})
	})

	Context("Update", func() {
		Context("instance labels", func() {
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

				serviceAccount = &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app.kubernetes.io/name":      instance.Name,
							"app.kubernetes.io/part-of":   "rabbitmq",
							"this-was-the-previous-label": "should-be-deleted",
						},
					},
				}
				err := serviceAccountBuilder.Update(serviceAccount)
				Expect(err).NotTo(HaveOccurred())
			})

			It("sets owner reference", func() {
				Expect(serviceAccount.OwnerReferences[0].Name).To(Equal(instance.Name))
			})

			It("adds labels from the CR", func() {
				testLabels(serviceAccount.Labels)
			})

			It("restores the default labels", func() {
				labels := serviceAccount.Labels
				Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
				Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
				Expect(labels["app.kubernetes.io/part-of"]).To(Equal("rabbitmq"))
			})

			It("deletes the labels that are removed from the CR", func() {
				Expect(serviceAccount.Labels).NotTo(HaveKey("this-was-the-previous-label"))
			})
		})

		Context("instance annotations", func() {
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

				serviceAccount = &corev1.ServiceAccount{
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
				err := serviceAccountBuilder.Update(serviceAccount)
				Expect(err).NotTo(HaveOccurred())
			})

			It("annotations on service account", func() {
				expectedAnnotations := map[string]string{
					"my-annotation":                 "i-like-this",
					"old-annotation":                "old-value",
					"im-here-to-stay.kubernetes.io": "for-a-while",
					"kubernetes.io/name":            "should-stay",
					"kubectl.kubernetes.io/name":    "should-stay",
					"k8s.io/name":                   "should-stay",
				}
				Expect(serviceAccount.Annotations).To(Equal(expectedAnnotations))
			})
		})
	})

	Context("UpdateMayRequireStsRecreate", func() {
		It("returns false", func() {
			Expect(serviceAccountBuilder.UpdateMayRequireStsRecreate()).To(BeFalse())
		})
	})
})
