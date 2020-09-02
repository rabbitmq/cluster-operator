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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/resource"
	. "github.com/rabbitmq/cluster-operator/internal/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			configMapBuilder = builder.RabbitmqPluginsConfigMap()
		})

		Context("Build", func() {
			var configMap *corev1.ConfigMap

			BeforeEach(func() {
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
		})
	})
})
