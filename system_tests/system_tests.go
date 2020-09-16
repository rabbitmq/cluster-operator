// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
//

package system_tests

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/ini.v1"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	statefulSetSuffix = "server"
	pluginsConfig     = "plugins-conf"
)

var _ = Describe("Operator", func() {
	var (
		namespace = MustHaveEnv("NAMESPACE")
		ctx       = context.Background()
	)

	Context("Publish and consume a message in a single node cluster", func() {
		var (
			cluster  *rabbitmqv1beta1.RabbitmqCluster
			hostname string
			port     string
			username string
			password string
		)

		BeforeEach(func() {
			one := int32(1)
			cluster = generateRabbitmqCluster(namespace, "basic-rabbit")
			cluster.Spec.Replicas = &one
			cluster.Spec.Service.Type = "NodePort"
			cluster.Spec.Resources = &corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]k8sresource.Quantity{},
				Limits:   map[corev1.ResourceName]k8sresource.Quantity{},
			}
			Expect(createRabbitmqCluster(ctx, rmqClusterClient, cluster)).To(Succeed())
			waitForRabbitmqRunning(cluster)

			hostname = kubernetesNodeIp(ctx, clientSet)
			port = rabbitmqNodePort(ctx, clientSet, cluster, "management")

			var err error
			username, password, err = getUsernameAndPassword(ctx, clientSet, cluster.Namespace, cluster.Name)
			Expect(err).NotTo(HaveOccurred())
			assertHttpReady(hostname, port)
		})

		AfterEach(func() {
			Expect(rmqClusterClient.Delete(context.TODO(), cluster)).To(Succeed())
		})

		It("works", func() {
			By("being able to create a test queue and publish a message", func() {
				response, err := alivenessTest(hostname, port, username, password)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Status).To(Equal("ok"))
			})

			By("having required plugins enabled", func() {
				_, err := kubectlExec(namespace,
					statefulSetPodName(cluster, 0),
					"rabbitmq-plugins",
					"is_enabled",
					"rabbitmq_management",
					"rabbitmq_peer_discovery_k8s",
					"rabbitmq_prometheus",
				)

				Expect(err).NotTo(HaveOccurred())
			})

			By("having status conditions", func() {
				output, err := kubectl(
					"-n",
					cluster.Namespace,
					"get",
					"rabbitmqclusters",
					cluster.Name,
					"-ojsonpath='{.status.conditions[?(@.type==\"AllReplicasReady\")].status}'",
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(output)).To(Equal("'True'"))

				output, err = kubectl(
					"-n",
					cluster.Namespace,
					"get",
					"rabbitmqclusters",
					cluster.Name,
					"-ojsonpath='{.status.conditions[?(@.type==\"ClusterAvailable\")].status}'",
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(output)).To(Equal("'True'"))
			})
		})
	})

	Context("RabbitMQ configurations", func() {
		var cluster *rabbitmqv1beta1.RabbitmqCluster

		BeforeEach(func() {
			cluster = generateRabbitmqCluster(namespace, "config-rabbit")
			cluster.Spec.Resources = &corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]k8sresource.Quantity{},
				Limits:   map[corev1.ResourceName]k8sresource.Quantity{},
			}

			Expect(createRabbitmqCluster(ctx, rmqClusterClient, cluster)).To(Succeed())
			waitForRabbitmqRunning(cluster)
		})

		AfterEach(func() {
			Expect(rmqClusterClient.Delete(context.TODO(), cluster)).To(Succeed())
		})

		It("keeps rabbitmq server related configurations up-to-date", func() {
			By("updating enabled plugins when additionalPlugins are modified", func() {
				// modify rabbitmqcluster.spec.rabbitmq.additionalPlugins
				Expect(updateRabbitmqCluster(ctx, rmqClusterClient, cluster.Name, cluster.Namespace, func(cluster *rabbitmqv1beta1.RabbitmqCluster) {
					cluster.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{"rabbitmq_top"}
				})).To(Succeed())

				getConfigMapAnnotations := func() map[string]string {
					configMapName := cluster.ChildResourceName(pluginsConfig)
					configMap, err := clientSet.CoreV1().ConfigMaps(cluster.Namespace).Get(ctx, configMapName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					return configMap.Annotations
				}
				Eventually(getConfigMapAnnotations, 10, 0.5).Should(
					HaveKey("rabbitmq.com/pluginsUpdatedAt"), "plugins ConfigMap should have been annotated")
				Eventually(getConfigMapAnnotations, 60, 1).Should(
					Not(HaveKey("rabbitmq.com/pluginsUpdatedAt")), "plugins ConfigMap annotation should have been removed")

				_, err := kubectlExec(namespace,
					statefulSetPodName(cluster, 0),
					"rabbitmq-plugins",
					"is_enabled",
					"rabbitmq_management",
					"rabbitmq_peer_discovery_k8s",
					"rabbitmq_prometheus",
					"rabbitmq_top",
				)
				Expect(err).ToNot(HaveOccurred())
			})

			By("updating the rabbitmq.conf file when additionalConfig are modified", func() {
				Expect(updateRabbitmqCluster(ctx, rmqClusterClient, cluster.Name, cluster.Namespace, func(cluster *rabbitmqv1beta1.RabbitmqCluster) {
					cluster.Spec.Rabbitmq.AdditionalConfig = `vm_memory_high_watermark_paging_ratio = 0.5
cluster_partition_handling = ignore
cluster_keepalive_interval = 10000`
				})).To(Succeed())

				// wait for statefulSet to be restarted
				waitForRabbitmqUpdate(cluster)

				// verify that rabbitmq.conf contains provided configurations
				output, err := kubectlExec(namespace,
					statefulSetPodName(cluster, 0),
					"cat",
					"/etc/rabbitmq/rabbitmq.conf",
				)
				Expect(err).NotTo(HaveOccurred())
				cfg, err := ini.Load(output)
				Expect(err).NotTo(HaveOccurred())
				cfgMap := cfg.Section("").KeysHash()
				Expect(cfgMap).To(HaveKeyWithValue("vm_memory_high_watermark_paging_ratio", "0.5"))
				Expect(cfgMap).To(HaveKeyWithValue("cluster_keepalive_interval", "10000"))
				Expect(cfgMap).To(HaveKeyWithValue("cluster_partition_handling", "ignore"))
			})

			By("updating the advanced.config file when advancedConfig are modifed", func() {
				Expect(updateRabbitmqCluster(ctx, rmqClusterClient, cluster.Name, cluster.Namespace, func(cluster *rabbitmqv1beta1.RabbitmqCluster) {
					cluster.Spec.Rabbitmq.AdvancedConfig = `[
  {rabbit, [{auth_backends, [rabbit_auth_backend_ldap]}]}
].`
				})).To(Succeed())

				// wait for statefulSet to be restarted
				waitForRabbitmqUpdate(cluster)

				output, err := kubectlExec(namespace,
					statefulSetPodName(cluster, 0),
					"cat",
					"/etc/rabbitmq/advanced.config",
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("[\n  {rabbit, [{auth_backends, [rabbit_auth_backend_ldap]}]}\n]."))
			})

			By("updating the rabbitmq-env.conf file when additionalConfig are modified", func() {
				Expect(updateRabbitmqCluster(ctx, rmqClusterClient, cluster.Name, cluster.Namespace, func(cluster *rabbitmqv1beta1.RabbitmqCluster) {
					cluster.Spec.Rabbitmq.EnvConfig = `USE_LONGNAME=true
CONSOLE_LOG=new`
				})).To(Succeed())

				// wait for statefulSet to be restarted
				waitForRabbitmqUpdate(cluster)

				// verify that rabbitmq-env.conf contains provided configurations
				output, err := kubectlExec(namespace,
					statefulSetPodName(cluster, 0),
					"cat",
					"/etc/rabbitmq/rabbitmq-env.conf",
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("USE_LONGNAME=true"))
				Expect(string(output)).Should(ContainSubstring("CONSOLE_LOG=new"))
			})
		})
	})

	Context("Persistence", func() {
		var (
			cluster  *rabbitmqv1beta1.RabbitmqCluster
			hostname string
			port     string
			username string
			password string
		)

		BeforeEach(func() {
			cluster = generateRabbitmqCluster(namespace, "persistence-rabbit")
			cluster.Spec.Service.Type = "NodePort"
			cluster.Spec.Resources = &corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]k8sresource.Quantity{},
				Limits:   map[corev1.ResourceName]k8sresource.Quantity{},
			}
			Expect(createRabbitmqCluster(ctx, rmqClusterClient, cluster)).To(Succeed())

			waitForRabbitmqRunning(cluster)

			hostname = kubernetesNodeIp(ctx, clientSet)
			port = rabbitmqNodePort(ctx, clientSet, cluster, "management")

			var err error
			username, password, err = getUsernameAndPassword(ctx, clientSet, cluster.Namespace, cluster.Name)
			Expect(err).NotTo(HaveOccurred())
			assertHttpReady(hostname, port)
		})

		AfterEach(func() {
			Expect(rmqClusterClient.Delete(context.TODO(), cluster)).To(Succeed())
		})

		It("persists messages after pod deletion", func() {
			By("publishing a message", func() {
				err := publishToQueue(hostname, port, username, password)
				Expect(err).NotTo(HaveOccurred())
			})

			By("consuming a message after RabbitMQ was restarted", func() {
				// We are asserting this in the BeforeEach. Is it necessary again here?
				assertHttpReady(hostname, port)

				message, err := getMessageFromQueue(hostname, port, username, password)
				Expect(err).NotTo(HaveOccurred())
				Expect(message.Payload).To(Equal("hello"))
			})

			By("setting owner reference to persistence volume claim successfully", func() {
				pvcName := "persistence-" + statefulSetPodName(cluster, 0)
				pvc, err := clientSet.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(pvc.OwnerReferences)).To(Equal(1))
				Expect(pvc.OwnerReferences[0].Name).To(Equal(cluster.Name))
			})
		})
	})

	Context("Clustering", func() {
		When("RabbitmqCluster is deployed with 3 nodes", func() {
			var cluster *rabbitmqv1beta1.RabbitmqCluster

			BeforeEach(func() {
				three := int32(3)
				cluster = generateRabbitmqCluster(namespace, "ha-rabbit")
				cluster.Spec.Replicas = &three
				cluster.Spec.Service.Type = "NodePort"
				cluster.Spec.Resources = &corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]k8sresource.Quantity{},
					Limits:   map[corev1.ResourceName]k8sresource.Quantity{},
				}
				Expect(createRabbitmqCluster(ctx, rmqClusterClient, cluster)).To(Succeed())
				waitForRabbitmqRunning(cluster)
			})

			AfterEach(func() {
				Expect(rmqClusterClient.Delete(context.TODO(), cluster)).To(Succeed())
			})

			It("works", func() {
				username, password, err := getUsernameAndPassword(ctx, clientSet, cluster.Namespace, cluster.Name)
				hostname := kubernetesNodeIp(ctx, clientSet)
				port := rabbitmqNodePort(ctx, clientSet, cluster, "management")
				Expect(err).NotTo(HaveOccurred())
				assertHttpReady(hostname, port)

				response, err := alivenessTest(hostname, port, username, password)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Status).To(Equal("ok"))
			})
		})
	})

	Context("TLS", func() {
		When("TLS is correctly configured", func() {
			var (
				cluster       *rabbitmqv1beta1.RabbitmqCluster
				hostname      string
				amqpsNodePort string
				username      string
				password      string
				caFilePath    string
			)

			BeforeEach(func() {
				cluster = generateRabbitmqCluster(namespace, "tls-test-rabbit")
				cluster.Spec.Service.Type = "NodePort"
				cluster.Spec.Resources = &corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]k8sresource.Quantity{},
					Limits:   map[corev1.ResourceName]k8sresource.Quantity{},
				}
				Expect(createRabbitmqCluster(ctx, rmqClusterClient, cluster)).To(Succeed())
				waitForRabbitmqRunning(cluster)

				// Passing a single hostname for certificate creation works because
				// the AMPQS client is connecting using the same hostname
				hostname = kubernetesNodeIp(ctx, clientSet)
				caFilePath = createTLSSecret("rabbitmq-tls-test-secret", namespace, hostname)

				// Update CR with TLS secret name
				Expect(updateRabbitmqCluster(ctx, rmqClusterClient, cluster.Name, cluster.Namespace, func(cluster *rabbitmqv1beta1.RabbitmqCluster) {
					cluster.Spec.TLS.SecretName = "rabbitmq-tls-test-secret"
				})).To(Succeed())
				waitForTLSUpdate(cluster)
				amqpsNodePort = rabbitmqNodePort(ctx, clientSet, cluster, "amqps")
			})

			AfterEach(func() {
				Expect(rmqClusterClient.Delete(context.TODO(), cluster)).To(Succeed())
				Expect(k8sDeleteSecret("rabbitmq-tls-test-secret", namespace)).To(Succeed())
			})

			It("talks amqps with RabbitMQ", func() {
				var err error
				username, password, err = getUsernameAndPassword(ctx, clientSet, "rabbitmq-system", "tls-test-rabbit")
				Expect(err).NotTo(HaveOccurred())

				// try to publish and consume a message on a amqps url
				sentMessage := "Hello Rabbitmq!"
				Expect(publishToQueueAMQPS(sentMessage, username, password, hostname, amqpsNodePort, caFilePath)).To(Succeed())

				recievedMessage, err := getMessageFromQueueAMQPS(username, password, hostname, amqpsNodePort, caFilePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(recievedMessage).To(Equal(sentMessage))
			})
		})

		When("the TLS secret does not exist", func() {
			cluster := generateRabbitmqCluster(namespace, "tls-test-rabbit-faulty")
			cluster.Spec.TLS = rabbitmqv1beta1.TLSSpec{SecretName: "tls-secret-does-not-exist"}

			It("reports a TLSError event with the reason", func() {
				Expect(createRabbitmqCluster(ctx, rmqClusterClient, cluster)).To(Succeed())
				assertTLSError(cluster)
				Expect(rmqClusterClient.Delete(context.TODO(), cluster)).To(Succeed())
			})
		})
	})

	When("(web) MQTT and STOMP plugins are enabled", func() {
		var (
			cluster  *rabbitmqv1beta1.RabbitmqCluster
			hostname string
			username string
			password string
		)

		BeforeEach(func() {
			instanceName := "mqtt-stomp-rabbit"
			cluster = generateRabbitmqCluster(namespace, instanceName)
			cluster.Spec.Resources = &corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]k8sresource.Quantity{},
				Limits:   map[corev1.ResourceName]k8sresource.Quantity{},
			}
			cluster.Spec.Service.Type = "NodePort"
			cluster.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{
				"rabbitmq_mqtt",
				"rabbitmq_web_mqtt",
				"rabbitmq_stomp",
			}
			Expect(createRabbitmqCluster(ctx, rmqClusterClient, cluster)).To(Succeed())
			waitForRabbitmqRunning(cluster)

			hostname = kubernetesNodeIp(ctx, clientSet)
			var err error
			username, password, err = getUsernameAndPassword(ctx, clientSet, "rabbitmq-system", instanceName)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(rmqClusterClient.Delete(context.TODO(), cluster)).To(Succeed())
		})

		It("publishes and consumes a message", func() {
			By("MQTT")
			publishAndConsumeMQTTMsg(hostname, rabbitmqNodePort(ctx, clientSet, cluster, "mqtt"), username, password, false)

			By("MQTT-over-WebSockets")
			publishAndConsumeMQTTMsg(hostname, rabbitmqNodePort(ctx, clientSet, cluster, "web-mqtt"), username, password, true)

			By("STOMP")
			publishAndConsumeSTOMPMsg(hostname, rabbitmqNodePort(ctx, clientSet, cluster, "stomp"), username, password)

			// github.com/go-stomp/stomp does not support STOMP-over-WebSockets
		})
	})
})
