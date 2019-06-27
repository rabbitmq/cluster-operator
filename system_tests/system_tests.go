package system_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes"
)

var _ = Describe("System tests", func() {
	var namespace, instanceName, statefulSetName, podname string
	var clientSet *kubernetes.Clientset
	var rabbitmqHostName, rabbitmqUsername, rabbitmqPassword string

	BeforeEach(func() {
		var err error
		namespace = MustHaveEnv("NAMESPACE")
		instanceName = "rabbitmqcluster-sample"
		statefulSetName = "p-" + instanceName
		podname = "p-rabbitmqcluster-sample-0"

		clientSet, err = createClientSet()
		Expect(err).NotTo(HaveOccurred())

		rabbitmqHostName = MustHaveEnv("SERVICE_HOST")

		rabbitmqUsername, err = getRabbitmqUsernameOrPassword(clientSet, namespace, instanceName, "rabbitmq-username")
		Expect(err).NotTo(HaveOccurred())

		rabbitmqPassword, err = getRabbitmqUsernameOrPassword(clientSet, namespace, instanceName, "rabbitmq-password")
		Expect(err).NotTo(HaveOccurred())
	})

	It("can create a test queue and push a message", func() {
		response, err := rabbitmqAlivenessTest(rabbitmqHostName, rabbitmqUsername, rabbitmqPassword)
		Expect(err).NotTo(HaveOccurred())
		Expect(response.Status).To(Equal("ok"))
	})

	Context("Plugin tests", func() {
		It("has required plugins enabled", func() {

			err := kubectlExec(namespace,
				podname,
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

	Context("ReadinessProbe tests", func() {
		It("checks whether the rabbitmq cluster is ready to serve traffic", func() {
			By("not publishing addresses after stopping Rabbitmq app", func() {

				// Run kubectl exec rabbitmqctl stop_app
				err := kubectlExec(namespace, podname, "rabbitmqctl", "stop_app")
				Expect(err).NotTo(HaveOccurred())

				// Check endpoints and expect addresses are not ready
				Eventually(func() int {
					return endpointPoller(clientSet, namespace, "rabbitmqcluster-service")
				}, 35, 3).Should(Equal(0))
			})

			By("publishing addresses after starting the Rabbitmq app", func() {
				err := kubectlExec(namespace, podname, "rabbitmqctl", "start_app")
				Expect(err).ToNot(HaveOccurred())

				// Check endpoints and expect addresses are ready
				Eventually(func() int {
					return endpointPoller(clientSet, namespace, "rabbitmqcluster-service")
				}, 35, 3).Should(BeNumerically(">", 0))
			})

		})
	})

	Context("when the RabbitmqCluster StatefulSet is delete", func() {
		It("reconciles the state, and the cluster is working again", func() {
			err := kubectlDelete(namespace, "statefulset", statefulSetName)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() string {
				response, _ := rabbitmqAlivenessTest(rabbitmqHostName, rabbitmqUsername, rabbitmqPassword)
				if response == nil {
					return ""
				}
				return response.Status
			}, 120, 5).Should(Equal("ok"))
		})
	})
})
