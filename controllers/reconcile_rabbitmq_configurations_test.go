package controllers_test

import (
	"time"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("Reconcile rabbitmq Configurations", func() {
	var (
		cluster          *rabbitmqv1beta1.RabbitmqCluster
		defaultNamespace = "default"
	)

	AfterEach(func() {
		Expect(client.Delete(ctx, cluster)).To(Succeed())
		waitForClusterDeletion(ctx, cluster, client)
	})

	DescribeTable("Server configurations updates", func(testCase string) {
		cluster = &rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: defaultNamespace,
				Name:      "rabbitmq-" + testCase,
			},
		}
		Expect(client.Create(ctx, cluster)).To(Succeed())
		waitForClusterCreation(ctx, cluster, client)

		// ensure that cfm and statefulSet does not have annotations set when cluster just got created
		cfm := configMap(ctx, cluster, "server-conf")
		Expect(cfm.Annotations).ShouldNot(HaveKey("rabbitmq.com/serverConfUpdatedAt"))
		sts := statefulSet(ctx, cluster)
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
			cfm := configMap(ctx, cluster, "server-conf")
			annotations = cfm.Annotations
			return annotations
		}, 5).Should(HaveKey("rabbitmq.com/serverConfUpdatedAt"))
		_, err := time.Parse(time.RFC3339, annotations["rabbitmq.com/serverConfUpdatedAt"])
		Expect(err).NotTo(HaveOccurred(), "Annotation rabbitmq.com/serverConfUpdatedAt was not a valid RFC3339 timestamp")

		By("annotating the sts podTemplate")
		// ensure statefulSet annotations
		Eventually(func() map[string]string {
			sts := statefulSet(ctx, cluster)
			annotations = sts.Spec.Template.Annotations
			return annotations
		}, 5).Should(HaveKey("rabbitmq.com/lastRestartAt"))
		_, err = time.Parse(time.RFC3339, annotations["rabbitmq.com/lastRestartAt"])
		Expect(err).NotTo(HaveOccurred(), "Annotation rabbitmq.com/lastRestartAt was not a valid RFC3339 timestamp")
	},

		Entry("spec.rabbitmq.additionalConfig is updated", "additional-config"),
		Entry("spec.rabbitmq.advancedConfig is updated", "advanced-config"),
		Entry("spec.rabbitmq.envConfig is updated", "env-config"),
	)

	Context("scale out", func() {
		It("does not restart StatefulSet", func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: defaultNamespace,
					Name:      "rabbitmq-scale-out",
				},
			}
			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)

			cfm := configMap(ctx, cluster, "server-conf")
			Expect(cfm.Annotations).ShouldNot(HaveKey("rabbitmq.com/serverConfUpdatedAt"))
			sts := statefulSet(ctx, cluster)
			Expect(sts.Annotations).ShouldNot(HaveKey("rabbitmq.com/lastRestartAt"))

			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				r.Spec.Replicas = pointer.Int32Ptr(5)
			})).To(Succeed())

			Consistently(func() map[string]string {
				return configMap(ctx, cluster, "server-conf").Annotations
			}, 3, 0.3).ShouldNot(HaveKey("rabbitmq.com/serverConfUpdatedAt"))

			Consistently(func() map[string]string {
				sts := statefulSet(ctx, cluster)
				return sts.Spec.Template.Annotations
			}, 3, 0.3).ShouldNot(HaveKey("rabbitmq.com/lastRestartAt"))
		})
	})
})
