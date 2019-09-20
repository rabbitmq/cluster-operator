package system_tests

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/pivotal/rabbitmq-for-kubernetes/internal/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	podCreationTimeout     = 300 * time.Second
	serviceCreationTimeout = 10 * time.Second
	serviceSuffix          = "ingress"
	statefulSetSuffix      = "server"
	configMapSuffix        = "server-conf"
)

var _ = Describe("Operator", func() {
	var (
		clientSet *kubernetes.Clientset
		k8sClient client.Client

		namespace = MustHaveEnv("NAMESPACE")
	)

	BeforeEach(func() {
		var err error
		clientSet, err = createClientSet()
		Expect(err).NotTo(HaveOccurred())
		k8sClient = mgr.GetClient()
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
			Expect(createRabbitmqCluster(k8sClient, cluster)).NotTo(HaveOccurred())

			waitForRabbitmqRunning(cluster)

			hostname = rabbitmqHostname(clientSet, cluster)

			var err error
			username, password, err = getRabbitmqUsernameAndPassword(clientSet, cluster.Namespace, cluster.Name)
			Expect(err).NotTo(HaveOccurred())

			assertHttpReady(hostname)
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.TODO(), cluster)).To(Succeed())
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

			By("updating the CR status correctly", func() {
				Eventually(func() error {
					err := clientSet.CoreV1().Pods(namespace).Delete(statefulSetPodName(cluster, 0), &metav1.DeleteOptions{})
					return err
				}, 10, 2).ShouldNot(HaveOccurred())

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
		})
	})

	Context("ReadinessProbe tests", func() {
		var (
			cluster     *rabbitmqv1beta1.RabbitmqCluster
			serviceName string
			podName     string
		)

		BeforeEach(func() {
			cluster = generateRabbitmqCluster(namespace, "readiness-rabbit")
			serviceName = cluster.ChildResourceName(serviceSuffix)
			podName = statefulSetPodName(cluster, 0)
			Expect(createRabbitmqCluster(k8sClient, cluster)).NotTo(HaveOccurred())

			assertStatefulSetReady(cluster)
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.TODO(), cluster)).To(Succeed())
		})

		It("checks whether the rabbitmq cluster is ready to serve traffic", func() {
			By("not publishing addresses after stopping Rabbitmq app", func() {
				_, err := kubectlExec(namespace, podName, "rabbitmqctl", "stop_app")
				Expect(err).NotTo(HaveOccurred())

				// Check endpoints and expect addresses are not ready
				Eventually(func() int {
					return endpointPoller(clientSet, namespace, serviceName)
				}, 120, 3).Should(Equal(0))
			})

			By("publishing addresses after starting the Rabbitmq app", func() {
				_, err := kubectlExec(namespace, podName, "rabbitmqctl", "start_app")
				Expect(err).ToNot(HaveOccurred())

				// Check endpoints and expect addresses are ready
				Eventually(func() int {
					return endpointPoller(clientSet, namespace, serviceName)
				}, 120, 3).Should(BeNumerically(">", 0))
			})
		})
	})

	When("the StatefulSet is deleted", func() {
		var cluster *rabbitmqv1beta1.RabbitmqCluster

		BeforeEach(func() {
			cluster = generateRabbitmqCluster(namespace, "statefulset-rabbit")
			Expect(createRabbitmqCluster(k8sClient, cluster)).NotTo(HaveOccurred())

			assertStatefulSetReady(cluster)
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.TODO(), cluster)).To(Succeed())
		})

		It("reconciles the state, and the cluster is working again", func() {
			err := kubectlDelete(namespace, "statefulset", cluster.ChildResourceName(statefulSetSuffix))
			Expect(err).NotTo(HaveOccurred())

			assertStatefulSetReady(cluster)
		})
	})

	When("the ConfigMap and Service resources are deleted", func() {
		var (
			cluster       *rabbitmqv1beta1.RabbitmqCluster
			configMapName string
			serviceName   string
		)

		BeforeEach(func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "delete-my-resources",
					Namespace: namespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Replicas: 1,
				},
			}

			configMapName = cluster.ChildResourceName(configMapSuffix)
			serviceName = cluster.ChildResourceName(serviceSuffix)

			Expect(k8sClient.Create(context.TODO(), cluster)).NotTo(HaveOccurred())

			Eventually(func() error {
				err := clientSet.CoreV1().ConfigMaps(namespace).Delete(configMapName, &metav1.DeleteOptions{})
				return err
			}, 30, 2).ShouldNot(HaveOccurred())

			Eventually(func() error {
				err := clientSet.CoreV1().Services(namespace).Delete(serviceName, &metav1.DeleteOptions{})
				return err
			}, 30, 2).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			err := k8sClient.Delete(context.TODO(), cluster)
			if err != nil {
				Expect(err.Error()).To(ContainSubstring("not found"))
			}
		})

		It("recreates the resources", func() {
			Eventually(func() error {
				_, err := clientSet.CoreV1().ConfigMaps(namespace).Get(configMapName, metav1.GetOptions{})
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("not found"))
				}
				return err
			}, 10).ShouldNot(HaveOccurred())
			Eventually(func() error {
				_, err := clientSet.CoreV1().Services(namespace).Get(serviceName, metav1.GetOptions{})
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("not found"))
				}
				return err
			}, 10).ShouldNot(HaveOccurred())
		})
	})

	When("using our gcr repository", func() {
		var cluster *rabbitmqv1beta1.RabbitmqCluster

		BeforeEach(func() {
			cluster = generateRabbitmqCluster(namespace, "image-rabbit")

			cluster.Spec.Image.Repository = "eu.gcr.io/cf-rabbitmq-for-k8s-bunny"
			cluster.Spec.ImagePullSecret = "gcr-viewer"
			Expect(createRabbitmqCluster(k8sClient, cluster)).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.TODO(), cluster)).To(Succeed())
		})

		It("successfully creates pods using private image and configured repository", func() {
			assertStatefulSetReady(cluster)
		})
	})

	When("a service type and annotations is configured in the manager configMap", func() {
		var (
			cluster                *rabbitmqv1beta1.RabbitmqCluster
			expectedConfigurations *config.Config
			serviceName            string
		)

		BeforeEach(func() {
			operatorConfigMapName := "p-rmq-operator-config"
			configMap, err := clientSet.CoreV1().ConfigMaps(namespace).Get(operatorConfigMapName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(configMap.Data["SERVICE"]).NotTo(BeNil())

			expectedConfigurations, err = config.NewConfig([]byte(configMap.Data["CONFIG"]))

			cluster = generateRabbitmqCluster(namespace, "nodeport-rabbit")
			serviceName = cluster.ChildResourceName(serviceSuffix)

			Expect(createRabbitmqCluster(k8sClient, cluster)).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.TODO(), cluster)).To(Succeed())
		})

		It("creates the service type and annotations as configured in manager config", func() {
			Eventually(func() string {
				svc, err := clientSet.CoreV1().Services(namespace).Get(serviceName, metav1.GetOptions{})
				if err != nil {
					Expect(err).To(MatchError(fmt.Sprintf("services \"%s\" not found", serviceName)))
					return ""
				}

				return string(svc.Spec.Type)
			}, serviceCreationTimeout).Should(Equal(expectedConfigurations.Service.Type))
			Eventually(func() map[string]string {
				svc, err := clientSet.CoreV1().Services(namespace).Get(serviceName, metav1.GetOptions{})
				if err != nil {
					Expect(err).To(MatchError(fmt.Sprintf("services \"%s\" not found", serviceName)))
					return nil
				}

				return svc.Annotations
			}, serviceCreationTimeout).Should(Equal(expectedConfigurations.Service.Annotations))
		})
	})

	Context("persistence", func() {
		var (
			cluster *rabbitmqv1beta1.RabbitmqCluster
			pvcName string
		)

		AfterEach(func() {
			err := k8sClient.Delete(context.TODO(), cluster)
			if !apierrors.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		})

		When("storage class name and storage is specified in the RabbitmqCluster Spec", func() {
			BeforeEach(func() {
				cluster = generateRabbitmqCluster(namespace, "persistence-storageclass-rabbit")
				pvcName = "persistence-" + statefulSetPodName(cluster, 0)

				// 'standard' is the default StorageClass in GCE
				cluster.Spec.Persistence.StorageClassName = "standard"
				cluster.Spec.Persistence.Storage = "2Gi"
				Expect(createRabbitmqCluster(k8sClient, cluster)).NotTo(HaveOccurred())

				waitForRabbitmqRunning(cluster)
			})

			It("creates the RabbitmqCluster with the specified storage", func() {
				pvList, err := clientSet.CoreV1().PersistentVolumes().List(metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				for _, pv := range pvList.Items {
					if pv.Spec.ClaimRef.Name == pvcName {
						storageCap := pv.Spec.Capacity["storage"]
						storageCapPointer := &storageCap
						Expect(pv.Spec.StorageClassName).To(Equal("standard"))
						Expect(storageCapPointer.String()).To(Equal("2Gi"))
					}
				}
			})
		})

		When("storage configuration is only specified in the operator configMap", func() {
			var hostname, username, password string

			BeforeEach(func() {
				cluster = generateRabbitmqCluster(namespace, "persistence-rabbit")
				pvcName = "persistence-" + statefulSetPodName(cluster, 0)

				cluster.Spec.Service.Type = "LoadBalancer"
				Expect(createRabbitmqCluster(k8sClient, cluster)).NotTo(HaveOccurred())

				waitForRabbitmqRunning(cluster)

				var err error
				username, password, err = getRabbitmqUsernameAndPassword(clientSet, namespace, cluster.Name)
				Expect(err).NotTo(HaveOccurred())

				hostname = rabbitmqHostname(clientSet, cluster)
				assertHttpReady(hostname)

				response, err := rabbitmqAlivenessTest(hostname, username, password)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Status).To(Equal("ok"))
			})

			It("works as expected", func() {
				By("creating the persistent volume using the configured storage class", func() {
					pvList, err := clientSet.CoreV1().PersistentVolumes().List(metav1.ListOptions{})
					Expect(err).NotTo(HaveOccurred())
					for _, pv := range pvList.Items {
						if pv.Spec.ClaimRef.Name == pvcName {
							Expect(pv.Spec.StorageClassName).To(Equal("persistent-test"))
						}
					}
				})

				By("successfully perserving messages after recreating a pod ", func() {
					err := rabbitmqPublishToNewQueue(hostname, username, password)
					Expect(err).NotTo(HaveOccurred())

					err = kubectlDelete(namespace, "pod", statefulSetPodName(cluster, 0))
					Expect(err).NotTo(HaveOccurred())

					waitForRabbitmqRunning(cluster)
					assertHttpReady(hostname)

					message, err := rabbitmqGetMessageFromQueue(hostname, username, password)
					Expect(err).NotTo(HaveOccurred())
					Expect(message.Payload).To(Equal("hello"))
				})

				By("deleting the persistent volume and claim when CRD is deleted", func() {
					Expect(k8sClient.Delete(context.TODO(), cluster)).To(Succeed())

					var err error
					Eventually(func() error {
						_, err = clientSet.CoreV1().PersistentVolumeClaims(namespace).Get(pvcName, metav1.GetOptions{})
						return err
					}, 20).Should(HaveOccurred())

					Expect(apierrors.IsNotFound(err)).To(BeTrue())
				})
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
				Expect(createRabbitmqCluster(k8sClient, cluster)).NotTo(HaveOccurred())

				assertStatefulSetReady(cluster)
			})

			AfterEach(func() {
				Expect(k8sClient.Delete(context.TODO(), cluster)).To(Succeed())
			})

			It("forms a working cluster", func() {
				pods := []string{
					statefulSetPodName(cluster, 0),
					statefulSetPodName(cluster, 1),
					statefulSetPodName(cluster, 2),
				}

				verifyClusterNodes(cluster, pods, "Running")
				verifyClusterNodes(cluster, pods, "Disk")

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
