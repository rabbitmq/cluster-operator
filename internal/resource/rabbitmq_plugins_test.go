// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance wit the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
//

package resource_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/resource"
	. "github.com/rabbitmq/cluster-operator/internal/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("RabbitMQPlugins", func() {

	Context("DesiredPlugins", func() {
		When("AdditionalPlugins is empty", func() {
			It("returns list of required plugins", func() {
				plugins := NewRabbitmqPlugins(nil)
				Expect(plugins.DesiredPlugins()).To(ConsistOf([]string{"rabbitmq_peer_discovery_k8s", "rabbitmq_prometheus", "rabbitmq_management"}))
			})
		})

		When("AdditionalPlugins are provided", func() {
			It("returns a concatenated list of plugins", func() {
				morePlugins := []rabbitmqv1beta1.Plugin{"rabbitmq_shovel", "my_great_plugin"}
				plugins := NewRabbitmqPlugins(morePlugins)

				Expect(plugins.DesiredPlugins()).To(ConsistOf([]string{"rabbitmq_peer_discovery_k8s",
					"rabbitmq_prometheus",
					"rabbitmq_management",
					"my_great_plugin",
					"rabbitmq_shovel",
				}))
			})
		})

		When("AdditionalPlugins are provided with duplicates", func() {
			It("returns a unique list of plugins", func() {
				morePlugins := []rabbitmqv1beta1.Plugin{"rabbitmq_management", "rabbitmq_shovel", "my_great_plugin", "rabbitmq_shovel"}
				plugins := NewRabbitmqPlugins(morePlugins)

				Expect(plugins.DesiredPlugins()).To(ConsistOf([]string{"rabbitmq_peer_discovery_k8s",
					"rabbitmq_prometheus",
					"rabbitmq_management",
					"my_great_plugin",
					"rabbitmq_shovel",
				}))
			})
		})
	})

	Context("PluginsConfigMap", func() {
		var (
			instance         rabbitmqv1beta1.RabbitmqCluster
			configMapBuilder *resource.RabbitmqPluginsConfigMapBuilder
			builder          *resource.RabbitmqResourceBuilder
			scheme           *runtime.Scheme
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
			configMapBuilder = builder.RabbitmqPluginsConfigMap()
		})

		Context("Build", func() {
			var configMap *corev1.ConfigMap

			BeforeEach(func() {
				instance.Labels = map[string]string{
					"app.kubernetes.io/foo": "bar",
					"foo":                   "bar",
					"rabbitmq":              "is-great",
					"foo/app.kubernetes.io": "edgecase",
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

				obj, err := configMapBuilder.Build()
				configMap = obj.(*corev1.ConfigMap)
				Expect(err).NotTo(HaveOccurred())
			})

			It("generates a ConfigMap with the correct name and namespace", func() {
				Expect(configMap.Name).To(Equal(builder.Instance.ChildResourceName("plugins-conf")))
				Expect(configMap.Namespace).To(Equal(builder.Instance.Namespace))
			})

			It("adds list of default plugins", func() {
				expectedEnabledPlugins := "[" +
					"rabbitmq_peer_discovery_k8s," +
					"rabbitmq_prometheus," +
					"rabbitmq_management]."

				obj, err := configMapBuilder.Build()
				Expect(err).NotTo(HaveOccurred())

				configMap = obj.(*corev1.ConfigMap)
				Expect(configMap.Data).To(HaveKeyWithValue("enabled_plugins", expectedEnabledPlugins))
			})

			It("adds labels from the instance and default labels", func() {
				Expect(configMap.Labels).To(SatisfyAll(
					HaveLen(6),
					HaveKeyWithValue("foo", "bar"),
					HaveKeyWithValue("rabbitmq", "is-great"),
					HaveKeyWithValue("foo/app.kubernetes.io", "edgecase"),
					HaveKeyWithValue("app.kubernetes.io/name", instance.Name),
					HaveKeyWithValue("app.kubernetes.io/component", "rabbitmq"),
					HaveKeyWithValue("app.kubernetes.io/part-of", "rabbitmq"),
					Not(HaveKey("app.kubernetes.io/foo")),
				))
			})

			It("adds annotations from the instance", func() {
				Expect(configMap.Annotations).To(SatisfyAll(
					HaveLen(1),
					HaveKeyWithValue("my-annotation", "i-like-this"),
					Not(HaveKey("kubernetes.io/name")),
					Not(HaveKey("kubectl.kubernetes.io/name")),
					Not(HaveKey("k8s.io/name")),
					Not(HaveKey("kubernetes.io/other")),
					Not(HaveKey("kubectl.kubernetes.io/other")),
					Not(HaveKey("k8s.io/other")),
				))
			})
		})

		Context("Update", func() {
			var configMap *corev1.ConfigMap

			BeforeEach(func() {
				configMap = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      instance.Name,
						Namespace: instance.Namespace,
					},
				}
			})

			It("sets owner reference", func() {
				instance = rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "rabbit1",
					},
				}
				Expect(configMapBuilder.Update(configMap)).NotTo(HaveOccurred())
				Expect(configMap.OwnerReferences[0].Name).To(Equal(instance.Name))
			})

			When("additionalPlugins are provided in instance spec", func() {
				When("no previous data is present", func() {
					It("creates data and sets enabled_plugins", func() {
						builder.Instance.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{"rabbitmq_management", "rabbitmq_management", "rabbitmq_shovel", "my_great_plugin"}

						expectedEnabledPlugins := "[" +
							"rabbitmq_peer_discovery_k8s," +
							"rabbitmq_prometheus," +
							"rabbitmq_management," +
							"rabbitmq_shovel," +
							"my_great_plugin]."

						err := configMapBuilder.Update(configMap)
						Expect(err).NotTo(HaveOccurred())
						Expect(configMap.Data).To(HaveKeyWithValue("enabled_plugins", expectedEnabledPlugins))
					})
				})

				When("previous data is present", func() {
					BeforeEach(func() {
						configMap.Data = map[string]string{
							"enabled_plugins": "[rabbitmq_peer_discovery_k8s,rabbitmq_shovel]",
						}
					})

					It("updates enabled_plugins with unique list of default and additionalPlugins", func() {
						builder.Instance.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{"rabbitmq_management", "rabbitmq_management", "rabbitmq_shovel", "my_great_plugin"}

						expectedEnabledPlugins := "[" +
							"rabbitmq_peer_discovery_k8s," +
							"rabbitmq_prometheus," +
							"rabbitmq_management," +
							"rabbitmq_shovel," +
							"my_great_plugin]."

						err := configMapBuilder.Update(configMap)
						Expect(err).NotTo(HaveOccurred())
						Expect(configMap.Data).To(HaveKeyWithValue("enabled_plugins", expectedEnabledPlugins))
					})
				})
			})

			// ensures that we are not unnecessarily running `rabbitmq-plugins set` when CR labels are updated
			It("does not update labels on the config map", func() {
				configMap.Labels = map[string]string{
					"app.kubernetes.io/name":      instance.Name,
					"app.kubernetes.io/component": "rabbitmq",
					"app.kubernetes.io/part-of":   "rabbitmq",
				}
				instance.Labels = map[string]string{
					"new-label": "test",
				}
				Expect(configMapBuilder.Update(configMap)).To(Succeed())
				Expect(configMap.Labels).To(SatisfyAll(
					HaveLen(3),
					HaveKeyWithValue("app.kubernetes.io/name", instance.Name),
					HaveKeyWithValue("app.kubernetes.io/component", "rabbitmq"),
					HaveKeyWithValue("app.kubernetes.io/part-of", "rabbitmq"),
					Not(HaveKey("new-label")),
				))
			})
			// ensures that we are not unnecessarily running `rabbitmq-plugins set` when CR annotations are updated
			It("does not update annotations on the config map", func() {
				instance.Annotations = map[string]string{
					"new-annotation": "test",
				}
				Expect(configMapBuilder.Update(configMap)).To(Succeed())
				Expect(configMap.Annotations).To(BeEmpty())
			})
		})

		Context("UpdateMayRequireStsRecreate", func() {
			It("returns false", func() {
				Expect(configMapBuilder.UpdateMayRequireStsRecreate()).To(BeFalse())
			})
		})
	})
})
