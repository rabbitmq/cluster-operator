package system_tests

import (
	"context"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const podCreationTimeout time.Duration = 180 * time.Second
const serviceCreationTimeout time.Duration = 10 * time.Second

var _ = Describe("System tests", func() {
	var namespace, instanceName, statefulSetName, serviceName, podName string
	var clientSet *kubernetes.Clientset
	var rabbitmqHostName, rabbitmqUsername, rabbitmqPassword string
	var err error

	BeforeEach(func() {
		namespace = MustHaveEnv("NAMESPACE")
		clientSet, err = createClientSet()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Initial RabbitmqCluster setup", func() {
		var basicRabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster

		BeforeEach(func() {
			instanceName = "basic-rabbit"
			serviceName = "p-" + instanceName
			podName = "p-" + instanceName + "-0"

			basicRabbitmqCluster = generateRabbitmqCluster(namespace, instanceName)
			basicRabbitmqCluster.Spec.Service.Type = "LoadBalancer"
			Expect(createRabbitmqCluster(k8sClient, basicRabbitmqCluster)).NotTo(HaveOccurred())

			Eventually(func() bool {
				rabbitmqUsername, rabbitmqPassword, err = getRabbitmqUsernameAndPassword(clientSet, namespace, instanceName, "rabbitmq-username")
				if err != nil {
					return false
				}
				return true
			}, 120, 5).Should(BeTrue())

			Eventually(func() string {
				rabbitmqHostName, err = getExternalIP(clientSet, namespace, serviceName)
				if err != nil {
					return ""
				}
				return rabbitmqHostName
			}, 300, 5).Should(Not(Equal("")))

			Expect(err).NotTo(HaveOccurred())

			Eventually(func() int {
				client := &http.Client{Timeout: 5 * time.Second}
				url := fmt.Sprintf("http://%s:15672", rabbitmqHostName)

				req, _ := http.NewRequest(http.MethodGet, url, nil)

				resp, err := client.Do(req)
				if err != nil {
					return 0
				}
				defer resp.Body.Close()

				return resp.StatusCode
			}, podCreationTimeout, 5).Should(Equal(200))
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.TODO(), basicRabbitmqCluster)).To(Succeed())
		})

		It("works", func() {

			By("being able to create a test queue and publish a message", func() {

				response, err := rabbitmqAlivenessTest(rabbitmqHostName, rabbitmqUsername, rabbitmqPassword)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Status).To(Equal("ok"))
			})

			By("having required plugins enabled", func() {
				err := kubectlExec(namespace,
					podName,
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
		})
	})

	Context("ReadinessProbe tests", func() {
		var readinessRabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster
		BeforeEach(func() {
			instanceName = "readiness-rabbit"
			serviceName = "p-" + instanceName
			podName = "p-" + instanceName + "-0"

			readinessRabbitmqCluster = generateRabbitmqCluster(namespace, instanceName)
			Expect(createRabbitmqCluster(k8sClient, readinessRabbitmqCluster)).NotTo(HaveOccurred())

			Eventually(func() string {
				podStatus, err := checkPodStatus(clientSet, namespace, podName)
				if err != nil {
					Expect(err).To(MatchError(fmt.Sprintf("pods \"%s\" not found", podName)))
				}
				return podStatus
			}, podCreationTimeout, 5).Should(ContainSubstring("ContainersReady True"))
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.TODO(), readinessRabbitmqCluster)).To(Succeed())
		})

		It("checks whether the rabbitmq cluster is ready to serve traffic", func() {
			By("not publishing addresses after stopping Rabbitmq app", func() {

				// Run kubectl exec rabbitmqctl stop_app
				err := kubectlExec(namespace, podName, "rabbitmqctl", "stop_app")
				Expect(err).NotTo(HaveOccurred())

				// Check endpoints and expect addresses are not ready
				Eventually(func() int {
					return endpointPoller(clientSet, namespace, serviceName)
				}, 120, 3).Should(Equal(0))
			})

			By("publishing addresses after starting the Rabbitmq app", func() {
				err := kubectlExec(namespace, podName, "rabbitmqctl", "start_app")
				Expect(err).ToNot(HaveOccurred())

				// Check endpoints and expect addresses are ready
				Eventually(func() int {
					return endpointPoller(clientSet, namespace, serviceName)
				}, 120, 3).Should(BeNumerically(">", 0))
			})

		})
	})

	Context("when the RabbitmqCluster StatefulSet is deleted", func() {
		var statefulsetRabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster
		BeforeEach(func() {
			instanceName = "statefulset-rabbit"
			serviceName = "p-" + instanceName
			statefulSetName = "p-" + instanceName
			podName = "p-" + instanceName + "-0"

			statefulsetRabbitmqCluster = generateRabbitmqCluster(namespace, instanceName)
			Expect(createRabbitmqCluster(k8sClient, statefulsetRabbitmqCluster)).NotTo(HaveOccurred())

			Eventually(func() string {
				podStatus, err := checkPodStatus(clientSet, namespace, podName)
				if err != nil {
					Expect(err).To(MatchError(fmt.Sprintf("pods \"%s\" not found", podName)))
				}
				return podStatus
			}, podCreationTimeout, 5).Should(ContainSubstring("ContainersReady True"))
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.TODO(), statefulsetRabbitmqCluster)).To(Succeed())
		})

		It("reconciles the state, and the cluster is working again", func() {
			err := kubectlDelete(namespace, "statefulset", statefulSetName)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() string {
				pod, err := clientSet.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
				if err != nil {
					Expect(err).To(MatchError(fmt.Sprintf("pods \"%s\" not found", podName)))
					return ""
				}

				return fmt.Sprintf("%v", pod.Status.Conditions)
			}, podCreationTimeout, 5).Should(ContainSubstring("ContainersReady True"))
		})
	})

	Context("when using our gcr repository for our Rabbitmq management image", func() {
		var imageRabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster

		BeforeEach(func() {
			instanceName = "image-rabbit"
			podName = "p-" + instanceName + "-0"

			imageRabbitmqCluster = generateRabbitmqCluster(namespace, instanceName)
			imageRabbitmqCluster.Spec.Image.Repository = "eu.gcr.io/cf-rabbitmq-for-k8s-bunny"
			imageRabbitmqCluster.Spec.ImagePullSecret = "gcr-viewer"
			Expect(createRabbitmqCluster(k8sClient, imageRabbitmqCluster)).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.TODO(), imageRabbitmqCluster)).To(Succeed())
		})

		It("successfully creates pods using private image and configured repository", func() {
			Eventually(func() string {
				podStatus, err := checkPodStatus(clientSet, namespace, podName)
				if err != nil {
					Expect(err).To(MatchError(fmt.Sprintf("pods \"%s\" not found", podName)))
				}
				return podStatus
			}, podCreationTimeout, 5).Should(ContainSubstring("ContainersReady True"))
		})
	})

	Context("when NodePort service type is specified in the manager configMap", func() {
		var rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster
		BeforeEach(func() {
			instanceName = "nodeport-rabbit"
			serviceName = "p-" + instanceName

			rabbitmqCluster = generateRabbitmqCluster(namespace, instanceName)
			Expect(createRabbitmqCluster(k8sClient, rabbitmqCluster)).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
		})

		It("creates a NodePort service", func() {
			Eventually(func() string {
				svc, err := clientSet.CoreV1().Services(namespace).Get(serviceName, metav1.GetOptions{})
				if err != nil {
					Expect(err).To(MatchError(fmt.Sprintf("services \"%s\" not found", serviceName)))
					return ""
				}

				return string(svc.Spec.Type)
			}, serviceCreationTimeout).Should(Equal("NodePort"))
		})
	})
})

func createRabbitmqCluster(client client.Client, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) error {
	return client.Create(context.TODO(), rabbitmqCluster)
}
