package controllers_test

import (
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Reconcile rabbitmq Configurations", func() {
	var (
		cluster          *rabbitmqv1beta1.RabbitmqCluster
		defaultNamespace = "default"
	)
	DescribeTable("Server configurations updates",
		func(testCase string) {
			// create rabbitmqcluster
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-" + testCase,
					Namespace: defaultNamespace,
				},
			}
			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)

			// ensure that configMap and statefulSet does not have annotations set when configurations haven't changed
			configMap, err := clientSet.CoreV1().ConfigMaps(cluster.Namespace).Get(ctx, cluster.ChildResourceName("server-conf"), metav1.GetOptions{})
			Expect(err).To(Not(HaveOccurred()))
			Expect(configMap.Annotations).ShouldNot(HaveKey("rabbitmq.com/serverConfUpdatedAt"))

			sts, err := clientSet.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, cluster.ChildResourceName("server"), metav1.GetOptions{})
			Expect(err).To(Not(HaveOccurred()))
			Expect(sts.Annotations).ShouldNot(HaveKey("rabbitmq.com/lastRestartAt"))

			// update rabbitmq server configurations
			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				if testCase == "additional-config" {
					r.Spec.Rabbitmq.AdditionalConfig = "test_config=0"
				}
				if testCase == "advanced-config" {
					r.Spec.Rabbitmq.AdvancedConfig = "sample-advanced-config."
				}
				if testCase == "env-config" {
					r.Spec.Rabbitmq.EnvConfig = "some-env-variable"
				}
			})).To(Succeed())

			By("annotating the server-conf ConfigMap")
			// ensure annotations from the server-conf ConfigMap
			var annotations map[string]string
			Eventually(func() map[string]string {
				configMap, err := clientSet.CoreV1().ConfigMaps(cluster.Namespace).Get(ctx, cluster.ChildResourceName("server-conf"), metav1.GetOptions{})
				Expect(err).To(Not(HaveOccurred()))
				annotations = configMap.Annotations
				return annotations
			}, 5).Should(HaveKey("rabbitmq.com/serverConfUpdatedAt"))
			_, err = time.Parse(time.RFC3339, annotations["rabbitmq.com/serverConfUpdatedAt"])
			Expect(err).NotTo(HaveOccurred(), "Annotation rabbitmq.com/serverConfUpdatedAt was not a valid RFC3339 timestamp")

			By("annotating the sts podTemplate")
			// ensure statefulSet annotations
			Eventually(func() map[string]string {
				sts, err := clientSet.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, cluster.ChildResourceName("server"), metav1.GetOptions{})
				Expect(err).To(Not(HaveOccurred()))
				annotations = sts.Spec.Template.Annotations
				return annotations
			}, 5).Should(HaveKey("rabbitmq.com/lastRestartAt"))
			_, err = time.Parse(time.RFC3339, annotations["rabbitmq.com/lastRestartAt"])
			Expect(err).NotTo(HaveOccurred(), "Annotation rabbitmq.com/lastRestartAt was not a valid RFC3339 timestamp")

			// delete rmq cluster
			Expect(client.Delete(ctx, cluster)).To(Succeed())
			waitForClusterDeletion(ctx, cluster, client)
		},

		Entry("spec.rabbitmq.additionalConfig is updated", "additional-config"),
		Entry("spec.rabbitmq.advancedConfig is updated", "advanced-config"),
		Entry("spec.rabbitmq.envConfig is updated", "env-config"),
	)
})
