package system_tests

import (
	"context"
	"k8s.io/apimachinery/pkg/types"
	"time"

	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ingressServiceSuffix = "ingress"
	statefulSetSuffix    = "server"
)

var _ = Describe("Operator", func() {
	var (
		clientSet *kubernetes.Clientset
		namespace = MustHaveEnv("NAMESPACE")
	)

	BeforeEach(func() {
		var err error
		clientSet, err = createClientSet()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Publish and consume a message", func() {
		var (
			cluster  *rabbitmqv1beta1.RabbitmqCluster
			hostname string
			username string
			password string
		)

		BeforeEach(func() {
			cluster = generateRabbitmqCluster(namespace, "basic-rabbit")
			cluster.Spec.Service.Type = "LoadBalancer"
			cluster.Spec.Image = "dev.registry.pivotal.io/p-rabbitmq-for-kubernetes/rabbitmq:latest"
			cluster.Spec.ImagePullSecret = "p-rmq-registry-access"
			cluster.Spec.Resources = &corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]k8sresource.Quantity{},
				Limits:   map[corev1.ResourceName]k8sresource.Quantity{},
			}
			Expect(createRabbitmqCluster(rmqClusterClient, cluster)).To(Succeed())

			waitForRabbitmqRunning(cluster)
			waitForLoadBalancer(clientSet, cluster)

			hostname = rabbitmqHostname(clientSet, cluster)

			var err error
			username, password, err = getRabbitmqUsernameAndPassword(clientSet, cluster.Namespace, cluster.Name)
			Expect(err).NotTo(HaveOccurred())
			assertHttpReady(hostname)
		})

		AfterEach(func() {
			Expect(rmqClusterClient.Delete(context.TODO(), cluster)).To(Succeed())
		})

		It("works", func() {
			By("being able to create a test queue and publish a message", func() {
				response, err := rabbitmqAlivenessTest(hostname, username, password)
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
			cluster.Spec.ImagePullSecret = "p-rmq-registry-access"
			cluster.Spec.Resources = &corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]k8sresource.Quantity{},
				Limits:   map[corev1.ResourceName]k8sresource.Quantity{},
			}

			Expect(createRabbitmqCluster(rmqClusterClient, cluster)).To(Succeed())
			waitForRabbitmqRunning(cluster)
		})

		AfterEach(func() {
			Expect(rmqClusterClient.Delete(context.TODO(), cluster)).To(Succeed())
		})

		It("keeps rabbitmq server related configurations up-to-date", func() {
			By("updating enabled plugins when additionalPlugins are modified", func() {
				// modify rabbitmqcluster.spec.rabbitmq.additionalPlugins
				fetchedRabbit := &rabbitmqv1beta1.RabbitmqCluster{}
				Expect(rmqClusterClient.Get(context.Background(), types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Name}, fetchedRabbit)).To(Succeed())
				fetchedRabbit.Spec.Rabbitmq.AdditionalPlugins = []rabbitmqv1beta1.Plugin{"rabbitmq_top"}
				Expect(rmqClusterClient.Update(context.TODO(), fetchedRabbit)).To(Succeed())

				Eventually(func() error {
					_, err := kubectlExec(namespace,
						statefulSetPodName(cluster, 0),
						"rabbitmq-plugins",
						"is_enabled",
						"rabbitmq_management",
						"rabbitmq_peer_discovery_k8s",
						"rabbitmq_prometheus",
						"rabbitmq_top",
					)
					return err
				}, 20*time.Second).To(Succeed())
			})

			By("updating the rabbitmq.conf file when additionalConfig are modified", func() {
				// modify rabbitmqcluster.spec.rabbitmq.additionalConfig
				fetchedRabbit := &rabbitmqv1beta1.RabbitmqCluster{}
				Expect(rmqClusterClient.Get(context.Background(), types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Name}, fetchedRabbit)).NotTo(HaveOccurred())
				fetchedRabbit.Spec.Rabbitmq.AdditionalConfig = `vm_memory_high_watermark_paging_ratio = 0.5
cluster_partition_handling = ignore
cluster_keepalive_interval = 10000`
				Expect(rmqClusterClient.Update(context.TODO(), fetchedRabbit)).To(Succeed())

				// wait for statefulSet to be restarted
				waitForStsRestart(clientSet, cluster.Namespace, cluster.ChildResourceName("server"))

				// verify that rabbitmq.conf contains provided configurations
				output, err := kubectlExec(namespace,
					statefulSetPodName(cluster, 0),
					"cat",
					"/etc/rabbitmq/rabbitmq.conf",
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(output)).Should(ContainSubstring("vm_memory_high_watermark_paging_ratio = 0.5"))
				Expect(string(output)).Should(ContainSubstring("cluster_keepalive_interval = 10000"))
				Expect(string(output)).Should(ContainSubstring("cluster_partition_handling = ignore"))
			})
		})
	})

	Context("Persistence", func() {
		var (
			cluster  *rabbitmqv1beta1.RabbitmqCluster
			hostname string
			username string
			password string
		)

		BeforeEach(func() {
			cluster = generateRabbitmqCluster(namespace, "persistence-rabbit")
			cluster.Spec.Service.Type = "LoadBalancer"
			cluster.Spec.Image = "dev.registry.pivotal.io/p-rabbitmq-for-kubernetes/rabbitmq:latest"
			cluster.Spec.ImagePullSecret = "p-rmq-registry-access"
			cluster.Spec.Resources = &corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]k8sresource.Quantity{},
				Limits:   map[corev1.ResourceName]k8sresource.Quantity{},
			}
			Expect(createRabbitmqCluster(rmqClusterClient, cluster)).To(Succeed())

			waitForRabbitmqRunning(cluster)
			waitForLoadBalancer(clientSet, cluster)

			hostname = rabbitmqHostname(clientSet, cluster)

			var err error
			username, password, err = getRabbitmqUsernameAndPassword(clientSet, cluster.Namespace, cluster.Name)
			Expect(err).NotTo(HaveOccurred())
			assertHttpReady(hostname)
		})

		AfterEach(func() {
			Expect(rmqClusterClient.Delete(context.TODO(), cluster)).To(Succeed())
		})

		It("persists messages after pod deletion", func() {
			By("publishing a message", func() {
				err := rabbitmqPublishToNewQueue(hostname, username, password)
				Expect(err).NotTo(HaveOccurred())
			})

			By("consuming a message after RabbitMQ was restarted", func() {
				assertHttpReady(hostname)

				message, err := rabbitmqGetMessageFromQueue(hostname, username, password)
				Expect(err).NotTo(HaveOccurred())
				Expect(message.Payload).To(Equal("hello"))
			})

			By("setting owner reference to persistence volume claim successfully", func() {
				pvcName := "persistence-" + statefulSetPodName(cluster, 0)
				pvc, err := clientSet.CoreV1().PersistentVolumeClaims(namespace).Get(pvcName, metav1.GetOptions{})
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
				cluster = generateRabbitmqCluster(namespace, "ha-rabbit")
				cluster.Spec.Replicas = 3
				cluster.Spec.Service.Type = "LoadBalancer"
				cluster.Spec.Resources = &corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]k8sresource.Quantity{},
					Limits:   map[corev1.ResourceName]k8sresource.Quantity{},
				}
				Expect(createRabbitmqCluster(rmqClusterClient, cluster)).To(Succeed())
			})

			AfterEach(func() {
				Expect(rmqClusterClient.Delete(context.TODO(), cluster)).To(Succeed())
			})

			It("works", func() {
				waitForRabbitmqRunning(cluster)
				username, password, err := getRabbitmqUsernameAndPassword(clientSet, cluster.Namespace, cluster.Name)
				waitForLoadBalancer(clientSet, cluster)
				hostname := rabbitmqHostname(clientSet, cluster)
				Expect(err).NotTo(HaveOccurred())

				response, err := rabbitmqAlivenessTest(hostname, username, password)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Status).To(Equal("ok"))
			})
		})
	})
})
