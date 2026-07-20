// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
//

package system_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/mod/semver"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	rabbithole "github.com/michaelklishin/rabbit-hole/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("Operator", func() {
	var (
		namespace = MustHaveEnv("NAMESPACE")
	)

	Context("single node cluster", func() {
		var (
			cluster  *rabbitmqv1beta1.RabbitmqCluster
			hostname string
			port     string
			username string
			password string
		)

		BeforeEach(func(ctx SpecContext) {
			cluster = newRabbitmqCluster(namespace, "basic-rabbit")
			Expect(createRabbitmqCluster(ctx, rmqClusterClient, cluster)).To(Succeed())
			waitForRabbitmqRunning(cluster)

			hostname, port = rabbitmqServiceEndpoint(ctx, clientSet, cluster, "management")

			var err error
			username, password, err = getUsernameAndPassword(ctx, clientSet, cluster.Namespace, cluster.Name)
			Expect(err).NotTo(HaveOccurred())
			assertHttpReady(ctx, hostname, port)
		})

		AfterEach(func(ctx SpecContext) {
			Expect(rmqClusterClient.Delete(ctx, cluster)).To(Succeed())
		})

		It("works", func(ctx SpecContext) {
			By("publishing and consuming a message", func() {
				response := alivenessTest(hostname, port, username, password)
				Expect(response.Status).To(Equal("ok"))
			})

			By("having required plugins enabled", func() {
				_, err := kubectlExec(namespace,
					statefulSetPodName(cluster, 0),
					"rabbitmq",
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

				Eventually(func() (string, error) {
					output, err := kubectl(
						"-n",
						cluster.Namespace,
						"get",
						"rabbitmqclusters",
						cluster.Name,
						"-ojsonpath='{.status.conditions[?(@.type==\"ClusterAvailable\")].status}'",
					)
					return string(output), err
				}, 30, 2).Should(Equal("'True'"))
			})

			By("setting observedGeneration", func() {
				fetchedRmq := &rabbitmqv1beta1.RabbitmqCluster{}
				Eventually(func() bool {
					Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, fetchedRmq)).To(Succeed())
					return fetchedRmq.Status.ObservedGeneration == fetchedRmq.Generation
				}, k8sQueryTimeout, 10).Should(BeTrue(), fmt.Sprintf("expected %d (Status.ObservedGeneration) = %d (Generation)",
					fetchedRmq.Status.ObservedGeneration, fetchedRmq.Generation))
			})

			By("having all feature flags enabled", func() {
				Eventually(func() []featureFlag {
					output, err := kubectlExec(namespace,
						statefulSetPodName(cluster, 0),
						"rabbitmq",
						"rabbitmqctl",
						"list_feature_flags",
						"--formatter=json",
					)
					Expect(err).NotTo(HaveOccurred())
					var flags []featureFlag
					Expect(json.Unmarshal(output, &flags)).To(Succeed())
					return flags
				}, 30, 2).
					Should(
						Or(
							ContainElement(
								MatchFields(IgnoreExtras, Fields{
									"State": BeEquivalentTo("enabled"),
								}),
							),
							ContainElement(
								MatchFields(IgnoreExtras, Fields{
									"Name":  Equal("khepri_db"),
									"State": BeEquivalentTo("disabled"),
								})), // temporary workaround since rabbitmq in main comes with Khepri disabled
						),
					)
			})
		}, SpecTimeout(time.Minute*3))
	})

	Context("RabbitMQ configurations", func() {
		var cluster *rabbitmqv1beta1.RabbitmqCluster

		BeforeEach(func(ctx SpecContext) {
			cluster = newRabbitmqCluster(namespace, "config-rabbit")
			Expect(createRabbitmqCluster(ctx, rmqClusterClient, cluster)).To(Succeed())
			waitForRabbitmqRunning(cluster)
		})

		AfterEach(func(ctx SpecContext) {
			Expect(rmqClusterClient.Delete(ctx, cluster)).To(Succeed())
		})

		It("keeps rabbitmq server related configurations up-to-date", func(ctx SpecContext) {
			By("updating enabled plugins  and the secret ports when additionalPlugins are modified", func() {
				// modify rabbitmqcluster.spec.rabbitmq.additionalPlugins
				Expect(updateRabbitmqCluster(ctx, rmqClusterClient, cluster.Name, cluster.Namespace, func(cluster *rabbitmqv1beta1.RabbitmqCluster) {
					cluster.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{"rabbitmq_top", "rabbitmq_mqtt"}
				})).To(Succeed())

				getConfigMapAnnotations := func() map[string]string {
					configMapName := cluster.ChildResourceName("plugins-conf")
					configMap, err := clientSet.CoreV1().ConfigMaps(cluster.Namespace).Get(ctx, configMapName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					return configMap.Annotations
				}
				Eventually(getConfigMapAnnotations, k8sQueryTimeout, 1).Should(
					HaveKey("rabbitmq.com/pluginsUpdatedAt"), "plugins ConfigMap should have been annotated")
				Eventually(getConfigMapAnnotations, 4*time.Minute, 1).Should(
					Not(HaveKey("rabbitmq.com/pluginsUpdatedAt")), "plugins ConfigMap annotation should have been removed")

				Eventually(func() map[string][]byte {
					secret, err := clientSet.CoreV1().Secrets(cluster.Namespace).Get(ctx, cluster.ChildResourceName("default-user"), metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					return secret.Data
				}, 30).Should(HaveKeyWithValue("mqtt-port", []byte("1883")))

				_, err := kubectlExec(namespace,
					statefulSetPodName(cluster, 0),
					"rabbitmq",
					"rabbitmq-plugins",
					"is_enabled",
					"rabbitmq_management",
					"rabbitmq_peer_discovery_k8s",
					"rabbitmq_prometheus",
					"rabbitmq_top",
					"rabbitmq_mqtt",
				)
				Expect(err).ToNot(HaveOccurred())
			})

			By("updating the userDefinedConfiguration.conf file when additionalConfig are modified", func() {
				Expect(updateRabbitmqCluster(ctx, rmqClusterClient, cluster.Name, cluster.Namespace, func(cluster *rabbitmqv1beta1.RabbitmqCluster) {
					cluster.Spec.Rabbitmq.AdditionalConfig = `cluster_partition_handling = ignore
cluster_keepalive_interval = 10000`
				})).To(Succeed())

				// wait for statefulSet to be restarted
				waitForRabbitmqUpdate(cluster)

				// verify that rabbitmq.conf contains provided configurations
				cfgMap := getConfigFileFromPod(namespace, cluster, "/etc/rabbitmq/conf.d/90-userDefinedConfiguration.conf")
				Expect(cfgMap).To(SatisfyAll(
					HaveKeyWithValue("cluster_keepalive_interval", "10000"),
					HaveKeyWithValue("cluster_partition_handling", "ignore"),
				))
			})

			By("updating the advanced.config file when advancedConfig are modified", func() {
				Expect(updateRabbitmqCluster(ctx, rmqClusterClient, cluster.Name, cluster.Namespace, func(cluster *rabbitmqv1beta1.RabbitmqCluster) {
					cluster.Spec.Rabbitmq.AdvancedConfig = `[
  {kernel, [{net_ticktime, 61}]}
].`
				})).To(Succeed())

				// wait for statefulSet to be restarted
				waitForRabbitmqUpdate(cluster)

				output, err := kubectlExec(namespace,
					statefulSetPodName(cluster, 0),
					"rabbitmq",
					"cat",
					"/etc/rabbitmq/advanced.config",
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("[\n  {kernel, [{net_ticktime, 61}]}\n]."))
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
					"rabbitmq",
					"cat",
					"/etc/rabbitmq/rabbitmq-env.conf",
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("USE_LONGNAME=true"))
				Expect(string(output)).Should(ContainSubstring("CONSOLE_LOG=new"))
			})
		}, SpecTimeout(time.Minute*10))
	})

	Context("Persistence", func() {
		var (
			cluster  *rabbitmqv1beta1.RabbitmqCluster
			hostname string
			port     string
			username string
			password string
		)

		BeforeEach(func(ctx SpecContext) {
			cluster = newRabbitmqCluster(namespace, "persistence-rabbit")
			Expect(createRabbitmqCluster(ctx, rmqClusterClient, cluster)).To(Succeed())

			waitForRabbitmqRunning(cluster)

			hostname, port = rabbitmqServiceEndpoint(ctx, clientSet, cluster, "management")

			var err error
			username, password, err = getUsernameAndPassword(ctx, clientSet, cluster.Namespace, cluster.Name)
			Expect(err).NotTo(HaveOccurred())
			assertHttpReady(ctx, hostname, port)
		})

		AfterEach(func(ctx SpecContext) {
			Expect(rmqClusterClient.Delete(ctx, cluster)).To(Succeed())
		})

		It("persists messages", FlakeAttempts(3), func(ctx SpecContext) {
			By("publishing a message", func() {
				Expect(publishToQueue(hostname, port, username, password)).To(Succeed())
			})

			By("deleting pod", func() {
				Expect(clientSet.CoreV1().Pods(namespace).Delete(ctx, statefulSetPodName(cluster, 0), metav1.DeleteOptions{})).To(Succeed())
				waitForRabbitmqUpdate(cluster)
			})

			By("consuming a message after RabbitMQ was restarted", func() {
				// We are asserting this in the BeforeEach. Is it necessary again here?
				assertHttpReady(ctx, hostname, port)

				message, err := getMessageFromQueue(hostname, port, username, password)
				Expect(err).NotTo(HaveOccurred())
				Expect(message.Payload).To(Equal("hello"))
			})

			By("setting owner reference to persistence volume claim successfully", func() {
				pvcName := "persistence-" + statefulSetPodName(cluster, 0)
				pvc, err := clientSet.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pvc.OwnerReferences).To(HaveLen(1))
				Expect(pvc.OwnerReferences[0].Name).To(Equal(cluster.Name))
			})
		}, SpecTimeout(time.Minute*3))

		It("retains PVCs when deleting a RabbitmqCluster with persistentVolumeClaimRetentionPolicy.whenDeleted=Retain", func(ctx SpecContext) {
			k8sVersion, err := clientSet.Discovery().ServerVersion()
			Expect(err).NotTo(HaveOccurred())
			if semver.Compare(k8sVersion.GitVersion, "v1.32.0") < 0 {
				Skip("StatefulSet persistentVolumeClaimRetentionPolicy is GA in Kubernetes 1.32+")
			}

			retainCluster := newRabbitmqCluster(namespace, fmt.Sprintf("retain-pvc-rabbit-%d", time.Now().UnixNano()))
			retainCluster.Spec.Override.StatefulSet = &rabbitmqv1beta1.StatefulSet{
				Spec: &rabbitmqv1beta1.StatefulSetSpec{
					PersistentVolumeClaimRetentionPolicy: &appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{
						WhenDeleted: appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
						WhenScaled:  appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
					},
				},
			}
			pvcName := retainCluster.PVCName(0)
			defer func() {
				_ = rmqClusterClient.Delete(ctx, retainCluster)
				_ = clientSet.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
			}()

			Expect(createRabbitmqCluster(ctx, rmqClusterClient, retainCluster)).To(Succeed())
			waitForRabbitmqRunning(retainCluster)
			By("creating a PVC without RabbitmqCluster owner references", func() {
				Eventually(func() []metav1.OwnerReference {
					pvc, err := clientSet.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					return pvc.OwnerReferences
				}, k8sQueryTimeout, 1).Should(BeEmpty())
			})

			By("deleting the RabbitmqCluster", func() {
				Expect(rmqClusterClient.Delete(ctx, retainCluster)).To(Succeed())
			})

			By("retaining the PVC after RabbitmqCluster deletion", func() {
				Consistently(func() error {
					_, err := clientSet.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
					return err
				}, 30*time.Second, 2*time.Second).Should(Succeed())
			})

		}, SpecTimeout(time.Minute*5))
	})

	Context("Persistence expansion", Label("persistence_expansion"), func() {
		var cluster *rabbitmqv1beta1.RabbitmqCluster

		AfterEach(func(ctx SpecContext) {
			Expect(rmqClusterClient.Delete(ctx, cluster)).To(Succeed())
		})

		BeforeEach(func(ctx SpecContext) {
			// volume expansion is not supported in kinD which is use in github action
			if !volumeExpansionSupported(ctx, clientSet) {
				Skip("default storageClass does not support volume expansion; skipping volume expansion test")
			}

			oldCapacity, _ := k8sresource.ParseQuantity("10Gi")

			cluster = newRabbitmqCluster(namespace, "resize-rabbit")
			cluster.Spec.Persistence = rabbitmqv1beta1.RabbitmqClusterPersistenceSpec{
				Storage: &oldCapacity,
			}
			Expect(createRabbitmqCluster(ctx, rmqClusterClient, cluster)).To(Succeed())
			waitForRabbitmqRunning(cluster)
		})

		It("allows volume expansion", func(ctx SpecContext) {
			podUID := pod(ctx, clientSet, cluster, 0).UID
			output, err := kubectlExec(namespace, statefulSetPodName(cluster, 0), "rabbitmq", "df", "/var/lib/rabbitmq/mnesia")
			Expect(err).ToNot(HaveOccurred())
			previousDiskSize, err := strconv.Atoi(strings.Fields(strings.Split(string(output), "\n")[1])[1])

			newCapacity, _ := k8sresource.ParseQuantity("12Gi")
			Expect(updateRabbitmqCluster(ctx, rmqClusterClient, cluster.Name, cluster.Namespace, func(cluster *rabbitmqv1beta1.RabbitmqCluster) {
				cluster.Spec.Persistence.Storage = &newCapacity
			})).To(Succeed())

			// PVC storage capacity updated
			Eventually(func() k8sresource.Quantity {
				pvcName := cluster.PVCName(0)
				pvc, err := clientSet.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				fmt.Printf("Retrieved PVC %s with conditions %+v\n", pvcName, pvc.Status.Conditions)

				return pvc.Spec.Resources.Requests["storage"]
			}, "10m", 10).Should(Equal(newCapacity))

			// storage capacity reflected in the pod
			Eventually(func() int {
				output, err = kubectlExec(namespace, statefulSetPodName(cluster, 0), "rabbitmq", "df", "/var/lib/rabbitmq/mnesia")
				Expect(err).ToNot(HaveOccurred())
				updatedDiskSize, err := strconv.Atoi(strings.Fields(strings.Split(string(output), "\n")[1])[1])
				Expect(err).ToNot(HaveOccurred())
				return updatedDiskSize
			}, "10m", 10).Should(BeNumerically(">", previousDiskSize))

			// pod was not recreated
			Expect(pod(ctx, clientSet, cluster, 0).UID).To(Equal(podUID))
		})
	})

	Context("Clustering", func() {
		When("RabbitmqCluster is deployed with 3 nodes", func() {
			var cluster *rabbitmqv1beta1.RabbitmqCluster

			BeforeEach(func(ctx SpecContext) {
				cluster = newRabbitmqCluster(namespace, "ha-rabbit")
				cluster.Spec.Replicas = ptr.To(int32(3))
				cluster.Spec.Resources = &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: k8sresource.MustParse("1Gi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: k8sresource.MustParse("1Gi"),
					},
				}

				Expect(createRabbitmqCluster(ctx, rmqClusterClient, cluster)).To(Succeed())
				waitForRabbitmqRunning(cluster)
			})

			AfterEach(func(ctx SpecContext) {
				Expect(rmqClusterClient.Delete(ctx, cluster)).To(Succeed())
			})

			It("works", func(ctx SpecContext) {
				username, password, err := getUsernameAndPassword(ctx, clientSet, cluster.Namespace, cluster.Name)
				hostname, port := rabbitmqServiceEndpoint(ctx, clientSet, cluster, "management")
				Expect(err).NotTo(HaveOccurred())
				assertHttpReady(ctx, hostname, port)

				Eventually(func() *HealthcheckResponse {
					return alivenessTest(hostname, port, username, password)
				}).Within(5 * time.Minute).ProbeEvery(20 * time.Second).Should(HaveField("Status", "ok"))

				// test https://github.com/rabbitmq/cluster-operator/issues/662 is fixed
				By("clustering correctly")
				testRabbitmqVersion := "v" + runningRabbitmqVersion(cluster)
				if semver.Compare(testRabbitmqVersion, "v3.8") >= 0 &&
					semver.Compare(testRabbitmqVersion, "v3.8.18") < 0 {
					Skip(testRabbitmqVersion + " is known to not cluster consistently (fixed in v3.8.18)")
				}
				rmqc, err := rabbithole.NewClient(fmt.Sprintf("http://%s:%s", hostname, port), username, password)
				Expect(err).NotTo(HaveOccurred())
				nodes, err := rmqc.ListNodes()
				Expect(err).NotTo(HaveOccurred())
				Expect(nodes).To(HaveLen(3))
			}, SpecTimeout(time.Minute*3))
		})
	})

	Context("TLS", func() {
		When("TLS is correctly configured and enforced", func() {
			var (
				cluster    *rabbitmqv1beta1.RabbitmqCluster
				hostname   string
				amqpsPort  string
				username   string
				password   string
				caFilePath string
			)

			BeforeEach(func(ctx SpecContext) {
				cluster = newRabbitmqCluster(namespace, "tls-test-rabbit")
				// Enable additional plugins that can share TLS config.
				cluster.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{
					"rabbitmq_mqtt",
					"rabbitmq_stomp",
				}
				cluster.Spec.Resources = &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: k8sresource.MustParse("1Gi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: k8sresource.MustParse("1Gi"),
					},
				}
				Expect(createRabbitmqCluster(ctx, rmqClusterClient, cluster)).To(Succeed())
				waitForRabbitmqRunning(cluster)

				// At this stage, the AMQPs listener is not enabled, because we haven't set the TLS secret name.
				// However, we need a hostname to create the TLS secret. In system tests, we access using the node IP
				// or the load balancer IP, which may not be known in advance.
				// As a workaround, we fetch the AMQP (plain) endpoint (i.e. node IP or load balancer IP) to create the TLS secret.
				// Note that the hostname/IP is the same for plain and TLS endpoints.
				hostname, _ = rabbitmqServiceEndpoint(ctx, clientSet, cluster, "amqp")
				var err error
				caFilePath, err = createTLSSecret("rabbitmq-tls-test-secret", namespace, hostname)
				Expect(err).ToNot(HaveOccurred())

				// Update RabbitmqCluster with TLS secret name
				Expect(updateRabbitmqCluster(ctx, rmqClusterClient, cluster.Name, cluster.Namespace, func(cluster *rabbitmqv1beta1.RabbitmqCluster) {
					cluster.Spec.TLS.SecretName = "rabbitmq-tls-test-secret"
					cluster.Spec.TLS.DisableNonTLSListeners = true
				})).To(Succeed())
				waitForRabbitmqUpdate(cluster)

				username, password, err = getUsernameAndPassword(ctx, clientSet, namespace, "tls-test-rabbit")
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(rmqClusterClient.Delete(ctx, cluster)).To(Succeed())
				Expect(k8sDeleteSecret("rabbitmq-tls-test-secret", namespace)).To(Succeed())
			})

			It("RabbitMQ responds to requests over secured protocols", func(ctx SpecContext) {
				By("talking AMQPS", func() {
					_, amqpsPort = rabbitmqServiceEndpoint(ctx, clientSet, cluster, "amqps")

					// try to publish and consume a message on using amqps
					sentMessage := "Hello Rabbitmq!"
					Expect(publishToQueueAMQPS(sentMessage, username, password, hostname, amqpsPort, caFilePath)).To(Succeed())
					receivedMessage, err := getMessageFromQueueAMQPS(username, password, hostname, amqpsPort, caFilePath)
					Expect(err).NotTo(HaveOccurred())
					Expect(receivedMessage).To(Equal(sentMessage))
				})

				By("connecting to management API over TLS", func() {
					_, managementTLSPort := rabbitmqServiceEndpoint(ctx, clientSet, cluster, "management-tls")
					Expect(connectHTTPS(username, password, hostname, managementTLSPort, caFilePath)).To(Succeed())
				})

				By("talking MQTTS", func() {
					var err error
					cfg := new(tls.Config)
					cfg.RootCAs = x509.NewCertPool()
					ca, err := os.ReadFile(caFilePath)
					Expect(err).NotTo(HaveOccurred())

					cfg.RootCAs.AppendCertsFromPEM(ca)
					_, mqttsPort := rabbitmqServiceEndpoint(ctx, clientSet, cluster, "mqtts")
					publishAndConsumeMQTTMsg(hostname, mqttsPort, username, password, false, cfg)
				})

				By("talking STOMPS", func() {
					var err error
					cfg := new(tls.Config)
					cfg.RootCAs = x509.NewCertPool()
					ca, err := os.ReadFile(caFilePath)
					Expect(err).NotTo(HaveOccurred())

					cfg.RootCAs.AppendCertsFromPEM(ca)
					_, stompsPort := rabbitmqServiceEndpoint(ctx, clientSet, cluster, "stomps")
					publishAndConsumeSTOMPMsg(hostname, stompsPort, username, password, cfg)
				})

				By("disabling non TLS listeners", func() {
					// verify that rabbitmq.conf contains listeners.tcp = none
					cfgMap := getConfigFileFromPod(namespace, cluster, "/etc/rabbitmq/conf.d/90-userDefinedConfiguration.conf")
					Expect(cfgMap).To(SatisfyAll(
						HaveKeyWithValue("listeners.tcp", "none"),
						HaveKeyWithValue("stomp.listeners.tcp", "none"),
						HaveKeyWithValue("mqtt.listeners.tcp", "none"),
						HaveKeyWithValue("management.ssl.port", "15671"),
						Not(HaveKey("management.tcp.port")),
						HaveKeyWithValue("prometheus.ssl.port", "15691"),
						Not(HaveKey("prometheus.tcp.port")),
					))

					// verify that only tls ports are exposed in service
					service, err := clientSet.CoreV1().Services(cluster.Namespace).Get(ctx, cluster.ChildResourceName(""), metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					ports := service.Spec.Ports
					Expect(ports).To(HaveLen(5))
					Expect(containsPort(ports, "amqps")).To(BeTrue())
					Expect(containsPort(ports, "management-tls")).To(BeTrue())
					Expect(containsPort(ports, "prometheus-tls")).To(BeTrue())
					Expect(containsPort(ports, "mqtts")).To(BeTrue())
					Expect(containsPort(ports, "stomps")).To(BeTrue())
				})
			}, SpecTimeout(time.Minute*5))
		})

		When("the TLS secret does not exist", func() {
			cluster := newRabbitmqCluster(namespace, "tls-test-rabbit-faulty")
			cluster.Spec.TLS = rabbitmqv1beta1.TLSSpec{SecretName: "tls-secret-does-not-exist"}

			It("reports a TLSError event with the reason", func(ctx SpecContext) {
				Expect(createRabbitmqCluster(ctx, rmqClusterClient, cluster)).To(Succeed())
				assertTLSError(cluster)
				Expect(rmqClusterClient.Delete(ctx, cluster)).To(Succeed())
			}, SpecTimeout(time.Minute*5))
		})
	})

	When("(web) MQTT, STOMP and stream are enabled", func() {
		var (
			cluster  *rabbitmqv1beta1.RabbitmqCluster
			hostname string
			username string
			password string
		)

		BeforeEach(func(ctx SpecContext) {
			instanceName := "mqtt-stomp-stream"
			cluster = newRabbitmqCluster(namespace, instanceName)
			cluster.Spec.Service.Type = getServiceType()
			cluster.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{
				"rabbitmq_mqtt",
				"rabbitmq_web_mqtt",
				"rabbitmq_stomp",
				"rabbitmq_stream",
			}
			cluster.Spec.Resources = &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: k8sresource.MustParse("1Gi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: k8sresource.MustParse("1Gi"),
				},
			}
			Expect(createRabbitmqCluster(ctx, rmqClusterClient, cluster)).To(Succeed())
			waitForRabbitmqRunning(cluster)
			waitForPortReadiness(cluster, 1883)  // mqtt
			waitForPortReadiness(cluster, 61613) // stomp

			hostname, _ = rabbitmqServiceEndpoint(ctx, clientSet, cluster, "mqtt")
			var err error
			username, password, err = getUsernameAndPassword(ctx, clientSet, namespace, instanceName)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(rmqClusterClient.Delete(ctx, cluster)).To(Succeed())
		})

		It("publishes and consumes a message", func(ctx SpecContext) {
			By("MQTT")
			_, mqttPort := rabbitmqServiceEndpoint(ctx, clientSet, cluster, "mqtt")
			publishAndConsumeMQTTMsg(hostname, mqttPort, username, password, false, nil)

			By("MQTT-over-WebSockets")
			_, webMqttPort := rabbitmqServiceEndpoint(ctx, clientSet, cluster, "web-mqtt")
			publishAndConsumeMQTTMsg(hostname, webMqttPort, username, password, true, nil)

			By("STOMP")
			_, stompPort := rabbitmqServiceEndpoint(ctx, clientSet, cluster, "stomp")
			publishAndConsumeSTOMPMsg(hostname, stompPort, username, password, nil)

			By("Streams")
			if !hasFeatureEnabled(cluster, "stream_queue") {
				Skip("rabbitmq_stream plugin is not supported by RabbitMQ image " + cluster.Spec.Image)
			} else {
				waitForPortConnectivity(cluster)
				waitForPortReadiness(cluster, 5552) // stream
				_, streamPort := rabbitmqServiceEndpoint(ctx, clientSet, cluster, "stream")
				publishAndConsumeStreamMsg(hostname, streamPort, username, password)
			}
		}, SpecTimeout(time.Minute*5))

	})

})
