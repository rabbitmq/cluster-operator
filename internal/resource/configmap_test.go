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
	"bytes"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/resource"
	"gopkg.in/ini.v1"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
)

func defaultRabbitmqConf(instanceName string) string {
	return iniString(`
cluster_formation.peer_discovery_backend = rabbit_peer_discovery_k8s
cluster_formation.k8s.host               = kubernetes.default
cluster_formation.k8s.address_type       = hostname
cluster_partition_handling               = pause_minority
queue_master_locator                     = min-masters
disk_free_limit.absolute                 = 2GB
cluster_formation.randomized_startup_delay_range.min = 0
cluster_formation.randomized_startup_delay_range.max = 60
cluster_name                             = ` + instanceName)
}

var _ = Describe("GenerateServerConfigMap", func() {
	var (
		instance         rabbitmqv1beta1.RabbitmqCluster
		configMapBuilder *resource.ServerConfigMapBuilder
		builder          *resource.RabbitmqResourceBuilder
		scheme           *runtime.Scheme
	)

	BeforeEach(func() {
		instance = generateRabbitmqCluster()
		instance.Spec.Resources.Limits = corev1.ResourceList{}

		scheme = runtime.NewScheme()
		Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
		Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())
		builder = &resource.RabbitmqResourceBuilder{
			Instance: &instance,
			Scheme:   scheme,
		}
		configMapBuilder = builder.ServerConfigMap()
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
			Expect(configMap.Name).To(Equal(builder.Instance.ChildResourceName("server-conf")))
			Expect(configMap.Namespace).To(Equal(builder.Instance.Namespace))
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
			instance.ObjectMeta.Name = "rabbit1"

			Expect(configMapBuilder.Update(configMap)).To(Succeed())
			Expect(configMap.OwnerReferences[0].Name).To(Equal(instance.Name))
		})

		It("returns the default rabbitmq configuration", func() {
			builder.Instance.Spec.Rabbitmq.AdditionalConfig = ""

			expectedConfiguration := defaultRabbitmqConf(builder.Instance.Name)

			Expect(configMapBuilder.Update(configMap)).To(Succeed())
			Expect(configMap.Data).To(HaveKeyWithValue("operatorDefaults.conf", expectedConfiguration))
		})

		When("valid userDefinedConfiguration is provided", func() {
			It("adds configurations in a new rabbitmq configuration", func() {
				userDefinedConfiguration := "cluster_formation.peer_discovery_backend = my-backend\n" +
					"my-config-property-0 = great-value"

				instance.Spec.Rabbitmq.AdditionalConfig = userDefinedConfiguration

				expectedConfiguration := iniString(userDefinedConfiguration)

				Expect(configMapBuilder.Update(configMap)).To(Succeed())
				Expect(configMap.Data).To(HaveKeyWithValue("userDefinedConfiguration.conf", expectedConfiguration))
			})
		})

		When("invalid userDefinedConfiguration is provided", func() {
			It("errors", func() {
				instance.Spec.Rabbitmq.AdditionalConfig = " = invalid"
				Expect(configMapBuilder.Update(configMap)).NotTo(Succeed())
			})
		})

		Context("advanced.config", func() {
			It("sets data.advancedConfig when provided", func() {
				instance.Spec.Rabbitmq.AdvancedConfig = "[my-awesome-config]."
				Expect(configMapBuilder.Update(configMap)).To(Succeed())
				Expect(configMap.Data).To(HaveKeyWithValue("advanced.config", "[my-awesome-config]."))
			})

			It("does set data.advancedConfig when empty", func() {
				instance.Spec.Rabbitmq.AdvancedConfig = ""
				Expect(configMapBuilder.Update(configMap)).To(Succeed())
				Expect(configMap.Data).ToNot(HaveKey("advanced.config"))
			})

			Context("advanced.config is set", func() {
				When("new advanced.config is empty", func() {
					It("removes advanced.config key from configMap", func() {
						instance.Spec.Rabbitmq.AdvancedConfig = "[my-awesome-config]."
						Expect(configMapBuilder.Update(configMap)).To(Succeed())
						Expect(configMap.Data).To(HaveKey("advanced.config"))

						instance.Spec.Rabbitmq.AdvancedConfig = ""
						Expect(configMapBuilder.Update(configMap)).To(Succeed())
						Expect(configMap.Data).ToNot(HaveKey("advanced.config"))
					})
				})
			})
		})

		Context("rabbitmq-env.conf", func() {
			It("creates and populates a rabbitmq-env.conf when envConfig is provided", func() {
				expectedRabbitmqEnvConf := `USE_LONGNAME=true
CONSOLE_LOG=new`

				instance.Spec.Rabbitmq.EnvConfig = `USE_LONGNAME=true
CONSOLE_LOG=new`

				Expect(configMapBuilder.Update(configMap)).To(Succeed())
				Expect(configMap.Data).To(HaveKeyWithValue("rabbitmq-env.conf", expectedRabbitmqEnvConf))
			})

			It("populates rabbitmq-env.conf to empty string when envConfig is empty", func() {
				instance.Spec.Rabbitmq.EnvConfig = ""
				Expect(configMapBuilder.Update(configMap)).To(Succeed())
				Expect(configMap.Data).ToNot(HaveKey("rabbitmq-env.conf"))
			})

			Context("rabbitmq-env.conf is set", func() {
				When("new envConf is empty", func() {
					It("removes rabbitmq-env.conf key from configMap", func() {
						instance.Spec.Rabbitmq.EnvConfig = `USE_LONGNAME=true`

						Expect(configMapBuilder.Update(configMap)).To(Succeed())
						Expect(configMap.Data).To(HaveKey("rabbitmq-env.conf"))

						instance.Spec.Rabbitmq.EnvConfig = ""
						Expect(configMapBuilder.Update(configMap)).To(Succeed())
						Expect(configMap.Data).ToNot(HaveKey("rabbitmq-env.conf"))
					})
				})
			})
		})

		Context("TLS", func() {
			It("adds TLS config when TLS is enabled", func() {
				instance.ObjectMeta.Name = "rabbit-tls"
				instance.Spec.TLS.SecretName = "tls-secret"

				expectedConfiguration := iniString(`ssl_options.certfile  = /etc/rabbitmq-tls/tls.crt
					ssl_options.keyfile   = /etc/rabbitmq-tls/tls.key
					listeners.ssl.default = 5671
					management.ssl.certfile   = /etc/rabbitmq-tls/tls.crt
					management.ssl.keyfile    = /etc/rabbitmq-tls/tls.key
					management.ssl.port       = 15671
					prometheus.ssl.certfile = /etc/rabbitmq-tls/tls.crt
					prometheus.ssl.keyfile  = /etc/rabbitmq-tls/tls.key
					prometheus.ssl.port     = 15691
					management.tcp.port     = 15672
					prometheus.tcp.port       = 15692`)

				Expect(configMapBuilder.Update(configMap)).To(Succeed())
				Expect(configMap.Data).To(HaveKeyWithValue("userDefinedConfiguration.conf", expectedConfiguration))
			})

			When("MQTT, STOMP, AMQP 1.0 and Stream plugins are enabled", func() {
				It("adds TLS config for the additional plugins", func() {
					additionalPlugins := []rabbitmqv1beta1.Plugin{"rabbitmq_mqtt", "rabbitmq_stomp", "rabbitmq_amqp_1_0", "rabbitmq_stream"}

					instance.ObjectMeta.Name = "rabbit-tls"
					instance.Spec.TLS.SecretName = "tls-secret"
					instance.Spec.Rabbitmq.AdditionalPlugins = additionalPlugins

					expectedConfiguration := iniString(`ssl_options.certfile  = /etc/rabbitmq-tls/tls.crt
						ssl_options.keyfile   = /etc/rabbitmq-tls/tls.key
						listeners.ssl.default = 5671
						management.ssl.certfile   = /etc/rabbitmq-tls/tls.crt
						management.ssl.keyfile    = /etc/rabbitmq-tls/tls.key
						management.ssl.port       = 15671
						prometheus.ssl.certfile = /etc/rabbitmq-tls/tls.crt
						prometheus.ssl.keyfile   = /etc/rabbitmq-tls/tls.key
						prometheus.ssl.port       = 15691
						management.tcp.port     = 15672
						prometheus.tcp.port       = 15692
						mqtt.listeners.ssl.default = 8883
						stomp.listeners.ssl.1 = 61614
						stream.listeners.ssl.default = 5551`)

					Expect(configMapBuilder.Update(configMap)).To(Succeed())
					Expect(configMap.Data).To(HaveKeyWithValue("userDefinedConfiguration.conf", expectedConfiguration))
				})
			})

			It("preserves user configuration over Operator generated settings", func() {
				instance.ObjectMeta.Name = "rabbit-tls-with-user-conf"
				instance.Spec.TLS.SecretName = "tls-secret"
				instance.Spec.Rabbitmq.AdditionalConfig = "listeners.ssl.default = 12345"

				expectedConfiguration := iniString(`ssl_options.certfile  = /etc/rabbitmq-tls/tls.crt
					ssl_options.keyfile   = /etc/rabbitmq-tls/tls.key
					listeners.ssl.default = 12345
					management.ssl.certfile   = /etc/rabbitmq-tls/tls.crt
					management.ssl.keyfile    = /etc/rabbitmq-tls/tls.key
					management.ssl.port       = 15671
					prometheus.ssl.certfile = /etc/rabbitmq-tls/tls.crt
					prometheus.ssl.keyfile  = /etc/rabbitmq-tls/tls.key
					prometheus.ssl.port     = 15691
					management.tcp.port     = 15672
					prometheus.tcp.port       = 15692`)

				Expect(configMapBuilder.Update(configMap)).To(Succeed())
				Expect(configMap.Data).To(HaveKeyWithValue("userDefinedConfiguration.conf", expectedConfiguration))
			})
		})

		Context("Mutual TLS", func() {
			It("adds TLS config when TLS is enabled", func() {
				instance.ObjectMeta.Name = "rabbit-tls"
				instance.Spec.TLS.SecretName = "tls-secret"
				instance.Spec.TLS.CaSecretName = "tls-mutual-secret"

				expectedConfiguration := iniString(`ssl_options.certfile   = /etc/rabbitmq-tls/tls.crt
					ssl_options.keyfile    = /etc/rabbitmq-tls/tls.key
					listeners.ssl.default  = 5671
					management.ssl.certfile   = /etc/rabbitmq-tls/tls.crt
					management.ssl.keyfile    = /etc/rabbitmq-tls/tls.key
					management.ssl.port       = 15671
					prometheus.ssl.certfile = /etc/rabbitmq-tls/tls.crt
					prometheus.ssl.keyfile   = /etc/rabbitmq-tls/tls.key
					prometheus.ssl.port       = 15691
					management.tcp.port     = 15672
					prometheus.tcp.port       = 15692
					ssl_options.cacertfile = /etc/rabbitmq-tls/ca.crt
					ssl_options.verify     = verify_peer
					management.ssl.cacertfile = /etc/rabbitmq-tls/ca.crt
					prometheus.ssl.cacertfile = /etc/rabbitmq-tls/ca.crt`)

				Expect(configMapBuilder.Update(configMap)).To(Succeed())
				Expect(configMap.Data).To(HaveKeyWithValue("userDefinedConfiguration.conf", expectedConfiguration))
			})

			When("Web MQTT and Web STOMP are enabled", func() {
				It("adds TLS config for the additional plugins", func() {
					additionalPlugins := []rabbitmqv1beta1.Plugin{"rabbitmq_web_mqtt", "rabbitmq_web_stomp"}

					instance.ObjectMeta.Name = "rabbit-tls"
					instance.Spec.TLS.SecretName = "tls-secret"
					instance.Spec.TLS.CaSecretName = "tls-mutual-secret"
					instance.Spec.Rabbitmq.AdditionalPlugins = additionalPlugins

					expectedConfiguration := iniString(`ssl_options.certfile   = /etc/rabbitmq-tls/tls.crt
						ssl_options.keyfile    = /etc/rabbitmq-tls/tls.key
						listeners.ssl.default  = 5671

						management.ssl.certfile   = /etc/rabbitmq-tls/tls.crt
						management.ssl.keyfile    = /etc/rabbitmq-tls/tls.key
						management.ssl.port       = 15671

						prometheus.ssl.certfile = /etc/rabbitmq-tls/tls.crt
						prometheus.ssl.keyfile   = /etc/rabbitmq-tls/tls.key
						prometheus.ssl.port       = 15691

						management.tcp.port       = 15672
						prometheus.tcp.port       = 15692

						ssl_options.cacertfile = /etc/rabbitmq-tls/ca.crt
						ssl_options.verify     = verify_peer
						management.ssl.cacertfile = /etc/rabbitmq-tls/ca.crt
						prometheus.ssl.cacertfile = /etc/rabbitmq-tls/ca.crt

						web_mqtt.ssl.port       = 15676
						web_mqtt.ssl.cacertfile = /etc/rabbitmq-tls/ca.crt
						web_mqtt.ssl.certfile   = /etc/rabbitmq-tls/tls.crt
						web_mqtt.ssl.keyfile    = /etc/rabbitmq-tls/tls.key

						web_stomp.ssl.port       = 15673
						web_stomp.ssl.cacertfile = /etc/rabbitmq-tls/ca.crt
						web_stomp.ssl.certfile   = /etc/rabbitmq-tls/tls.crt
						web_stomp.ssl.keyfile    = /etc/rabbitmq-tls/tls.key`)

					Expect(configMapBuilder.Update(configMap)).To(Succeed())
					Expect(configMap.Data).To(HaveKeyWithValue("userDefinedConfiguration.conf", expectedConfiguration))
				})
			})
		})

		When("DisableNonTLSListeners is set to true", func() {
			It("disables non tls listeners for rabbitmq and management plugin", func() {
				instance = rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "rabbit-tls",
					},
					Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
						TLS: rabbitmqv1beta1.TLSSpec{
							SecretName:             "some-secret",
							DisableNonTLSListeners: true,
						},
					},
				}

				expectedConfiguration := iniString(`ssl_options.certfile  = /etc/rabbitmq-tls/tls.crt
					ssl_options.keyfile   = /etc/rabbitmq-tls/tls.key
					listeners.ssl.default = 5671

					management.ssl.certfile   = /etc/rabbitmq-tls/tls.crt
					management.ssl.keyfile    = /etc/rabbitmq-tls/tls.key
					management.ssl.port       = 15671

					prometheus.ssl.certfile = /etc/rabbitmq-tls/tls.crt
					prometheus.ssl.keyfile   = /etc/rabbitmq-tls/tls.key
					prometheus.ssl.port       = 15691

					listeners.tcp = none`)

				Expect(configMapBuilder.Update(configMap)).To(Succeed())
				Expect(configMap.Data).To(HaveKeyWithValue("userDefinedConfiguration.conf", expectedConfiguration))
			})

			It("disables non tls listeners for mqtt, stomp and stream plugins if enabled", func() {
				instance = rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "rabbit-tls",
					},
					Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
						TLS: rabbitmqv1beta1.TLSSpec{
							SecretName:             "some-secret",
							DisableNonTLSListeners: true,
						},
						Rabbitmq: rabbitmqv1beta1.RabbitmqClusterConfigurationSpec{
							AdditionalPlugins: []rabbitmqv1beta1.Plugin{
								"rabbitmq_mqtt",
								"rabbitmq_stomp",
								"rabbitmq_stream",
							},
						},
					},
				}

				expectedConfiguration := iniString(`ssl_options.certfile   = /etc/rabbitmq-tls/tls.crt
					ssl_options.keyfile    = /etc/rabbitmq-tls/tls.key
					listeners.ssl.default  = 5671

					management.ssl.certfile   = /etc/rabbitmq-tls/tls.crt
					management.ssl.keyfile    = /etc/rabbitmq-tls/tls.key
					management.ssl.port       = 15671

					prometheus.ssl.certfile = /etc/rabbitmq-tls/tls.crt
					prometheus.ssl.keyfile   = /etc/rabbitmq-tls/tls.key
					prometheus.ssl.port       = 15691

					listeners.tcp = none

					mqtt.listeners.ssl.default = 8883
					mqtt.listeners.tcp   = none

					stomp.listeners.ssl.1 = 61614
					stomp.listeners.tcp   = none

					stream.listeners.ssl.default = 5551
					stream.listeners.tcp = none`)

				Expect(configMapBuilder.Update(configMap)).To(Succeed())
				Expect(configMap.Data).To(HaveKeyWithValue("userDefinedConfiguration.conf", expectedConfiguration))
			})

			It("disables non tls listeners for web mqtt and web stomp when enabled", func() {
				instance = rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "rabbit-tls",
					},
					Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
						TLS: rabbitmqv1beta1.TLSSpec{
							SecretName:             "some-secret",
							CaSecretName:           "some-mutual-secret",
							DisableNonTLSListeners: true,
						},
						Rabbitmq: rabbitmqv1beta1.RabbitmqClusterConfigurationSpec{
							AdditionalPlugins: []rabbitmqv1beta1.Plugin{
								"rabbitmq_web_mqtt",
								"rabbitmq_web_stomp",
							},
						},
					},
				}

				expectedConfiguration := iniString(`ssl_options.certfile   = /etc/rabbitmq-tls/tls.crt
					ssl_options.keyfile    = /etc/rabbitmq-tls/tls.key
					listeners.ssl.default  = 5671

					management.ssl.certfile = /etc/rabbitmq-tls/tls.crt
					management.ssl.keyfile  = /etc/rabbitmq-tls/tls.key
					management.ssl.port     = 15671

					prometheus.ssl.certfile = /etc/rabbitmq-tls/tls.crt
					prometheus.ssl.keyfile  = /etc/rabbitmq-tls/tls.key
					prometheus.ssl.port     = 15691

					listeners.tcp = none

					ssl_options.cacertfile = /etc/rabbitmq-tls/ca.crt
					ssl_options.verify     = verify_peer
					management.ssl.cacertfile = /etc/rabbitmq-tls/ca.crt
					prometheus.ssl.cacertfile = /etc/rabbitmq-tls/ca.crt

					web_mqtt.ssl.port       = 15676
					web_mqtt.ssl.cacertfile = /etc/rabbitmq-tls/ca.crt
					web_mqtt.ssl.certfile   = /etc/rabbitmq-tls/tls.crt
					web_mqtt.ssl.keyfile    = /etc/rabbitmq-tls/tls.key
					web_mqtt.tcp.listener = none

					web_stomp.ssl.port       = 15673
					web_stomp.ssl.cacertfile = /etc/rabbitmq-tls/ca.crt
					web_stomp.ssl.certfile   = /etc/rabbitmq-tls/tls.crt
					web_stomp.ssl.keyfile    = /etc/rabbitmq-tls/tls.key
					web_stomp.tcp.listener = none`)

				Expect(configMapBuilder.Update(configMap)).To(Succeed())
				Expect(configMap.Data).To(HaveKeyWithValue("userDefinedConfiguration.conf", expectedConfiguration))
			})
		})

		Context("Memory Limits", func() {
			It("sets a RabbitMQ memory limit with headroom when memory limits are specified", func() {
				const GiB int64 = 1073741824
				instance.ObjectMeta.Name = "rabbit-mem-limit"
				instance.Spec.Resources.Limits = map[corev1.ResourceName]k8sresource.Quantity{corev1.ResourceMemory: k8sresource.MustParse("10Gi")}

				expectedConfiguration := iniString(fmt.Sprintf("total_memory_available_override_value = %d", 8*GiB))

				Expect(configMapBuilder.Update(configMap)).To(Succeed())
				Expect(configMap.Data).To(HaveKeyWithValue("userDefinedConfiguration.conf", expectedConfiguration))
			})
		})

		// this is to ensure that pods are not restarted when instance labels are updated
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

		// this is to ensure that pods are not restarted when instance annotations are updated
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

// iniString formats the input string using "gopkg.in/ini.v1"
func iniString(input string) string {
	ini.PrettySection = false
	var output bytes.Buffer
	cfg, _ := ini.Load([]byte(input))
	_, err := cfg.WriteTo(&output)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return output.String()
}
