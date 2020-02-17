package system_tests

import (
	"context"
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
	podCreationTimeout     = 360 * time.Second
	serviceCreationTimeout = 10 * time.Second
	ingressServiceSuffix   = "ingress"
	statefulSetSuffix      = "server"
	configMapSuffix        = "server-conf"
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

	Context("Initial RabbitmqCluster setup", func() {
		var (
			cluster  *rabbitmqv1beta1.RabbitmqCluster
			hostname string
			username string
			password string
		)

		BeforeEach(func() {
			cluster = generateRabbitmqCluster(namespace, "basic-rabbit")
			cluster.Spec.Service.Type = "LoadBalancer"
			cluster.Spec.Image = "registry.pivotal.io/p-rabbitmq-for-kubernetes-staging/rabbitmq:latest"
			cluster.Spec.ImagePullSecret = "p-rmq-registry-access"
			cluster.Spec.Resources = &corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]k8sresource.Quantity{},
				Limits:   map[corev1.ResourceName]k8sresource.Quantity{},
			}
			Expect(createRabbitmqCluster(rmqClusterClient, cluster)).NotTo(HaveOccurred())

			waitForRabbitmqRunning(cluster)

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
					"rabbitmq_federation",
					"rabbitmq_federation_management",
					"rabbitmq_management",
					"rabbitmq_peer_discovery_common",
					"rabbitmq_peer_discovery_k8s",
					"rabbitmq_shovel",
					"rabbitmq_shovel_management",
					"rabbitmq_prometheus",
				)

				Expect(err).NotTo(HaveOccurred())
			})

			By("publishing a message", func() {
				err := rabbitmqPublishToNewQueue(hostname, username, password)
				Expect(err).NotTo(HaveOccurred())
			})

			By("updating the CR status correctly", func() {
				Expect(clientSet.CoreV1().Pods(namespace).Delete(statefulSetPodName(cluster, 0), &metav1.DeleteOptions{})).NotTo(HaveOccurred())

				Eventually(func() []byte {
					output, err := kubectl(
						"-n",
						cluster.Namespace,
						"get",
						"rabbitmqclusters",
						cluster.Name,
						"-o=jsonpath='{.status.clusterStatus}'",
					)
					Expect(err).NotTo(HaveOccurred())
					return output

				}, 20, 1).Should(ContainSubstring("created"))

				waitForRabbitmqRunning(cluster)
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
				Expect(createRabbitmqCluster(rmqClusterClient, cluster)).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				Expect(rmqClusterClient.Delete(context.TODO(), cluster)).To(Succeed())
			})

			It("works", func() {
				waitForRabbitmqRunning(cluster)
				username, password, err := getRabbitmqUsernameAndPassword(clientSet, cluster.Namespace, cluster.Name)
				hostname := rabbitmqHostname(clientSet, cluster)
				Expect(err).NotTo(HaveOccurred())

				response, err := rabbitmqAlivenessTest(hostname, username, password)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Status).To(Equal("ok"))
			})
		})
	})
})
