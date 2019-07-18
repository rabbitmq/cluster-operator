package system_tests

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const podCreationTimeout time.Duration = 120 * time.Second

var _ = Describe("System tests", func() {
	var namespace, instanceName, statefulSetName, serviceName, podName string
	var clientSet *kubernetes.Clientset
	var rabbitmqHostName, rabbitmqUsername, rabbitmqPassword string

	BeforeEach(func() {
		var err error
		namespace = MustHaveEnv("NAMESPACE")
		instanceName = "rabbitmqcluster-sample"
		statefulSetName = "p-" + instanceName
		serviceName = "p-" + instanceName
		podName = "p-rabbitmqcluster-sample-0"

		clientSet, err = createClientSet()
		Expect(err).NotTo(HaveOccurred())

		rabbitmqUsername, err = getRabbitmqUsernameOrPassword(clientSet, namespace, instanceName, "rabbitmq-username")
		Expect(err).NotTo(HaveOccurred())

		rabbitmqPassword, err = getRabbitmqUsernameOrPassword(clientSet, namespace, instanceName, "rabbitmq-password")
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() string {
			rabbitmqHostName, err = getExternalIP(clientSet, namespace, serviceName)
			if err != nil {
				return ""
			}
			return rabbitmqHostName
		}, 60, 5).Should(Not(Equal("")))
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

	Context("ReadinessProbe tests", func() {
		It("checks whether the rabbitmq cluster is ready to serve traffic", func() {
			By("not publishing addresses after stopping Rabbitmq app", func() {

				// Run kubectl exec rabbitmqctl stop_app
				err := kubectlExec(namespace, podName, "rabbitmqctl", "stop_app")
				Expect(err).NotTo(HaveOccurred())

				// Check endpoints and expect addresses are not ready
				Eventually(func() int {
					return endpointPoller(clientSet, namespace, serviceName)
				}, 35, 3).Should(Equal(0))
			})

			By("publishing addresses after starting the Rabbitmq app", func() {
				err := kubectlExec(namespace, podName, "rabbitmqctl", "start_app")
				Expect(err).ToNot(HaveOccurred())

				// Check endpoints and expect addresses are ready
				Eventually(func() int {
					return endpointPoller(clientSet, namespace, serviceName)
				}, 35, 3).Should(BeNumerically(">", 0))
			})

		})
	})

	Context("when the RabbitmqCluster StatefulSet is deleted", func() {
		It("reconciles the state, and the cluster is working again", func() {
			err := kubectlDelete(namespace, "statefulset", statefulSetName)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() string {
				response, _ := rabbitmqAlivenessTest(rabbitmqHostName, rabbitmqUsername, rabbitmqPassword)
				if response == nil {
					return ""
				}
				return response.Status
			}, podCreationTimeout, 5).Should(Equal("ok"))
		})
	})

	Context("when using our gcr repository for our Rabbitmq management image", func() {
		var (
			client          client.Client
			rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster
		)

		BeforeEach(func() {
			scheme := runtime.NewScheme()
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).NotTo(HaveOccurred())
			Expect(defaultscheme.AddToScheme(scheme)).NotTo(HaveOccurred())

			config, err := createRestConfig()
			Expect(err).NotTo(HaveOccurred())

			mgr, err := ctrl.NewManager(config, ctrl.Options{Scheme: scheme})
			Expect(err).NotTo(HaveOccurred())
			client = mgr.GetClient()
		})

		AfterEach(func() {
			Expect(client.Delete(context.TODO(), rabbitmqCluster)).To(Succeed())
			Eventually(func() string {
				_, err := clientSet.CoreV1().Pods(namespace).Get("p-rabbitmq-one-0", metav1.GetOptions{})
				if err != nil {
					return err.Error()
				}
				return ""
			}, podCreationTimeout, 5).Should(ContainSubstring(`pods "p-rabbitmq-one-0" not found`))
		})

		It("successfully creates pods using private image and configured repository", func() {
			// we are relying on the `make destroy/destroy-ci` to cleanup the state
			// so that we have a chance to debug if it failed locally and in the ci
			rabbitmqCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-one",
					Namespace: namespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Plan: "single",
					Image: rabbitmqv1beta1.RabbitmqClusterImageSpec{
						Repository: "eu.gcr.io/cf-rabbitmq-for-k8s-bunny",
					},
					ImagePullSecret: "gcr-viewer",
				},
			}
			Expect(createRabbitmqCluster(client, rabbitmqCluster)).NotTo(HaveOccurred())

			Eventually(func() string {
				pod, err := clientSet.CoreV1().Pods(namespace).Get("p-rabbitmq-one-0", metav1.GetOptions{})
				if err != nil {
					Expect(err).To(MatchError(`pods "p-rabbitmq-one-0" not found`))
					return ""
				}

				return fmt.Sprintf("%v", pod.Status.Conditions)
			}, podCreationTimeout, 5).Should(ContainSubstring("ContainersReady True"))
		})
	})
})

func createRabbitmqCluster(client client.Client, rabbitmqCluster *rabbitmqv1beta1.RabbitmqCluster) error {
	return client.Create(context.TODO(), rabbitmqCluster)
}
