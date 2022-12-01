package system_tests

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("RabbitMQ Cluster with TLS enabled", func() {
	var (
		namespace        = MustHaveEnv("NAMESPACE")
		ctx              = context.Background()
		targetCluster    *rabbitmqv1beta1.RabbitmqCluster
		targetClusterRef rabbitmqv1beta1.RabbitmqClusterReference
		policy           rabbitmqv1beta1.Policy
		exchange         rabbitmqv1beta1.Exchange
		connectionSecret *corev1.Secret
	)

	BeforeEach(func() {
		targetCluster = basicTestRabbitmqCluster("tls-cluster", namespace)
		targetCluster.Spec.TLS.SecretName = tlsSecretName
		targetCluster.Spec.TLS.DisableNonTLSListeners = true
		setupTestRabbitmqCluster(rmqClusterClient, targetCluster)
		targetClusterRef = rabbitmqv1beta1.RabbitmqClusterReference{Name: targetCluster.Name}

		user, pass, err := getUsernameAndPassword(ctx, clientSet, targetCluster.Namespace, targetCluster.Name)
		Expect(err).NotTo(HaveOccurred(), "failed to get user and pass")
		connectionSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "uri-secret",
				Namespace: namespace,
			},
			StringData: map[string]string{
				"username": user,
				"password": pass,
				"uri":      "https://tls-cluster.rabbitmq-system.svc:15671",
			},
		}
		Expect(rmqClusterClient.Create(ctx, connectionSecret, &client.CreateOptions{})).To(Succeed())
		Eventually(func() string {
			output, err := kubectl(
				"-n",
				namespace,
				"get",
				"secrets",
				connectionSecret.Name,
			)
			if err != nil {
				Expect(string(output)).To(ContainSubstring("NotFound"))
			}
			return string(output)
		}, 10).Should(ContainSubstring("uri-secret"))
	})

	AfterEach(func() {
		Expect(rmqClusterClient.Delete(ctx, &policy)).To(Succeed())
		Eventually(func() string {
			output, _ := kubectl(
				"-n",
				policy.Namespace,
				"get",
				"policy",
				policy.Name,
			)
			return string(output)
		}, 90, 10).Should(ContainSubstring("NotFound"))
		Expect(rmqClusterClient.Delete(ctx, &exchange)).To(Succeed())
		Eventually(func() string {
			output, _ := kubectl(
				"-n",
				exchange.Namespace,
				"get",
				"exchange",
				exchange.Name,
			)
			return string(output)
		}, 90, 10).Should(ContainSubstring("NotFound"))
		Expect(rmqClusterClient.Delete(ctx, &rabbitmqv1beta1.RabbitmqCluster{ObjectMeta: metav1.ObjectMeta{Name: targetCluster.Name, Namespace: targetCluster.Namespace}})).To(Succeed())
		Eventually(func() string {
			output, _ := kubectl(
				"-n",
				targetCluster.Namespace,
				"get",
				"rabbitmqclusters",
				targetCluster.Name,
			)
			return string(output)
		}, 90, 10).Should(ContainSubstring("NotFound"))
		Expect(rmqClusterClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: connectionSecret.Name, Namespace: targetCluster.Namespace}})).To(Succeed())
		Expect(rmqClusterClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName, Namespace: targetCluster.Namespace}})).To(Succeed())
		Expect(rmqClusterClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName + "-ca", Namespace: targetCluster.Namespace}})).To(Succeed())
	})

	It("works", func() {
		By("successfully creating object when rabbitmqClusterReference.name is set")
		policy = rabbitmqv1beta1.Policy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "policy-tls-test",
				Namespace: namespace,
			},
			Spec: rabbitmqv1beta1.PolicySpec{
				Name:    "policy-tls-test",
				Pattern: ".*",
				ApplyTo: "queues",
				Definition: &runtime.RawExtension{
					Raw: []byte(`{"ha-mode":"all"}`),
				},
				RabbitmqClusterReference: targetClusterRef,
			},
		}
		Expect(rmqClusterClient.Create(ctx, &policy)).To(Succeed())

		var fetchedPolicy rabbitmqv1beta1.Policy
		Eventually(func() []rabbitmqv1beta1.Condition {
			Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace}, &fetchedPolicy)).To(Succeed())
			return fetchedPolicy.Status.Conditions
		}, waitUpdatedStatusCondition, 2).Should(HaveLen(1), "policy status condition should be present")

		readyCondition := fetchedPolicy.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))

		Eventually(func() string {
			output, err := kubectlExec(namespace,
				targetCluster.ChildResourceName("server")+"-0",
				"rabbitmq",
				"rabbitmqctl",
				"list_policies",
			)
			Expect(err).NotTo(HaveOccurred())
			return string(output)
		}, 30, 2).Should(ContainSubstring("policy-tls-test"))

		By("successfully creating object when rabbitmqClusterReference.connectionSecret is set")
		exchange = rabbitmqv1beta1.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tls-test",
				Namespace: namespace,
			},
			Spec: rabbitmqv1beta1.ExchangeSpec{
				Name:       "tls-test",
				Type:       "direct",
				AutoDelete: false,
				Durable:    true,
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					ConnectionSecret: &corev1.LocalObjectReference{Name: connectionSecret.Name},
				},
			},
		}
		Expect(rmqClusterClient.Create(ctx, &exchange)).To(Succeed())

		var fetched rabbitmqv1beta1.Exchange
		Eventually(func() []rabbitmqv1beta1.Condition {
			Expect(rmqClusterClient.Get(ctx, types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace}, &fetched)).To(Succeed())
			return fetched.Status.Conditions
		}, 10, 2).Should(HaveLen(1))

		readyCondition = fetched.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))

		Eventually(func() string {
			output, err := kubectlExec(namespace,
				targetCluster.ChildResourceName("server")+"-0",
				"rabbitmq",
				"rabbitmqctl",
				"list_exchanges",
			)
			Expect(err).NotTo(HaveOccurred())
			return string(output)
		}, 30, 2).Should(ContainSubstring("tls-test"))
	})
})
