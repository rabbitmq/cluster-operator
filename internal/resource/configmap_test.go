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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GenerateServerConfigMap", func() {
	var (
		instance         rabbitmqv1beta1.RabbitmqCluster
		configMapBuilder *resource.ServerConfigMapBuilder
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
		configMapBuilder = builder.ServerConfigMap()
	})

	Context("Build", func() {
		var configMap *corev1.ConfigMap

		BeforeEach(func() {
			obj, err := configMapBuilder.Build()
			configMap = obj.(*corev1.ConfigMap)
			Expect(err).NotTo(HaveOccurred())
		})

		It("generates a ConfigMap with the correct name and namespace", func() {
			Expect(configMap.Name).To(Equal(builder.Instance.ChildResourceName("server-conf")))
			Expect(configMap.Namespace).To(Equal(builder.Instance.Namespace))
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

		When("additionalConfig is not provided", func() {
			It("returns the default rabbitmq conf", func() {
				builder.Instance.Spec.Rabbitmq.AdditionalConfig = ""
				defaultRabbitmqConf := `cluster_formation.peer_discovery_backend        = rabbit_peer_discovery_k8s
cluster_formation.k8s.host                      = kubernetes.default
cluster_formation.k8s.address_type              = hostname
cluster_formation.node_cleanup.interval         = 30
cluster_formation.node_cleanup.only_log_warning = true
cluster_partition_handling                      = pause_minority
queue_master_locator                            = min-masters
cluster_name                                    = ` + builder.Instance.Name + "\n"
				Expect(configMapBuilder.Update(configMap)).To(Succeed())
				Expect(configMap.Data).To(HaveKeyWithValue("rabbitmq.conf", defaultRabbitmqConf))
			})
		})

		When("valid additionalConfig is provided", func() {
			BeforeEach(func() {
				instance.Spec.Rabbitmq.AdditionalConfig = `cluster_formation.peer_discovery_backend = my-backend
my-config-property-0 = great-value
my-config-property-1 = better-value`
			})

			It("appends configurations to the default rabbitmq.conf and overwrites duplicate keys", func() {
				expectedRabbitmqConf := `cluster_formation.peer_discovery_backend        = my-backend
cluster_formation.k8s.host                      = kubernetes.default
cluster_formation.k8s.address_type              = hostname
cluster_formation.node_cleanup.interval         = 30
cluster_formation.node_cleanup.only_log_warning = true
cluster_partition_handling                      = pause_minority
queue_master_locator                            = min-masters
cluster_name                                    = ` + builder.Instance.Name + `
my-config-property-0                            = great-value
my-config-property-1                            = better-value
`

				Expect(configMapBuilder.Update(configMap)).To(Succeed())
				Expect(configMap.Data).To(HaveKeyWithValue("rabbitmq.conf", expectedRabbitmqConf))
			})
		})

		When("invalid additionalConfig is provided", func() {
			BeforeEach(func() {
				instance.Spec.Rabbitmq.AdditionalConfig = " = invalid"
			})

			It("errors", func() {
				Expect(configMapBuilder.Update(configMap)).To(MatchError(
					"failed to append spec.rabbitmq.additionalConfig: error creating new key: empty key name"))
			})
		})

		Context("advanced.config", func() {
			It("sets data.advancedConfig when provided", func() {
				instance.Spec.Rabbitmq.AdvancedConfig = `
[
  {rabbit, [{auth_backends, [rabbit_auth_backend_ldap]}]}
].`
				Expect(configMapBuilder.Update(configMap)).To(Succeed())
				Expect(configMap.Data).To(HaveKeyWithValue("advanced.config", "\n[\n  {rabbit, [{auth_backends, [rabbit_auth_backend_ldap]}]}\n]."))
			})

			It("does set data.advancedConfig when empty", func() {
				instance.Spec.Rabbitmq.AdvancedConfig = ""
				Expect(configMapBuilder.Update(configMap)).To(Succeed())
				Expect(configMap.Data).ToNot(HaveKey("advanced.config"))
			})

			Context("advanced.config is set", func() {
				When("new advanced.config is empty", func() {
					It("removes advanced.config key from configMap", func() {
						instance.Spec.Rabbitmq.AdvancedConfig = `
[
  {rabbit, [{auth_backends, [rabbit_auth_backend_ldap]}]}
].`
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
				instance = rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "rabbit-tls",
					},
					Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
						TLS: rabbitmqv1beta1.TLSSpec{
							SecretName: "tls-secret",
						},
					},
				}

				Expect(configMapBuilder.Update(configMap)).To(Succeed())
				Expect(configMap.Data).To(HaveKeyWithValue("rabbitmq.conf", ContainSubstring(`
ssl_options.certfile                            = /etc/rabbitmq-tls/tls.crt
ssl_options.keyfile                             = /etc/rabbitmq-tls/tls.key
listeners.ssl.default                           = 5671`)))
			})
		})

		Context("Mutual TLS", func() {
			It("adds TLS config when TLS is enabled", func() {
				instance = rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "rabbit-tls",
					},
					Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
						TLS: rabbitmqv1beta1.TLSSpec{
							SecretName:   "tls-secret",
							CaSecretName: "tls-mutual-secret",
							CaCertName:   "ca.certificate",
						},
					},
				}

				Expect(configMapBuilder.Update(configMap)).To(Succeed())
				Expect(configMap.Data).To(HaveKeyWithValue("rabbitmq.conf", ContainSubstring(`
ssl_options.certfile                            = /etc/rabbitmq-tls/tls.crt
ssl_options.keyfile                             = /etc/rabbitmq-tls/tls.key
listeners.ssl.default                           = 5671
ssl_options.cacertfile                          = /etc/rabbitmq-tls/ca.certificate
ssl_options.verify                              = verify_peer`)))
			})
		})

		Context("labels", func() {
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

				configMap = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app.kubernetes.io/name":      instance.Name,
							"app.kubernetes.io/part-of":   "rabbitmq",
							"this-was-the-previous-label": "should-be-deleted",
						},
					},
				}
				err := configMapBuilder.Update(configMap)
				Expect(err).NotTo(HaveOccurred())
			})

			It("adds labels from the CR", func() {
				testLabels(configMap.Labels)
			})

			It("restores the default labels", func() {
				labels := configMap.Labels
				Expect(labels["app.kubernetes.io/name"]).To(Equal(instance.Name))
				Expect(labels["app.kubernetes.io/component"]).To(Equal("rabbitmq"))
				Expect(labels["app.kubernetes.io/part-of"]).To(Equal("rabbitmq"))
			})

			It("deletes the labels that are removed from the CR", func() {
				Expect(configMap.Labels).NotTo(HaveKey("this-was-the-previous-label"))
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

				configMap = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"my-annotation":                 "i-will-not-stay",
							"old-annotation":                "old-value",
							"im-here-to-stay.kubernetes.io": "for-a-while",
							"kubernetes.io/name":            "should-stay",
							"kubectl.kubernetes.io/name":    "should-stay",
							"k8s.io/name":                   "should-stay",
						},
					},
				}
				err := configMapBuilder.Update(configMap)
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates config map annotations", func() {
				expectedAnnotations := map[string]string{
					"my-annotation":                 "i-like-this",
					"old-annotation":                "old-value",
					"im-here-to-stay.kubernetes.io": "for-a-while",
					"kubernetes.io/name":            "should-stay",
					"kubectl.kubernetes.io/name":    "should-stay",
					"k8s.io/name":                   "should-stay",
				}

				Expect(configMap.Annotations).To(Equal(expectedAnnotations))
			})
		})
	})
})
