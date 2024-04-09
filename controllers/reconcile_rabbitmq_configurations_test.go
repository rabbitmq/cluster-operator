package controllers_test

import (
	"strings"
	"time"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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
					Name:      "rabbitmq-" + strings.ToLower(testCase),
					Namespace: defaultNamespace,
				},
			}
			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)

			// ensure that cfm and statefulSet does not have annotations set when configurations haven't changed
			cfm := configMap(ctx, cluster, "server-conf")
			Expect(cfm.Annotations).ShouldNot(HaveKey("rabbitmq.com/serverConfUpdatedAt"))

			sts := statefulSet(ctx, cluster)
			Expect(sts.Annotations).ShouldNot(HaveKey("rabbitmq.com/lastRestartAt"))

			// update rabbitmq server configurations
			Expect(updateWithRetry(cluster, func(r *rabbitmqv1beta1.RabbitmqCluster) {
				if testCase == "additionalConfig" {
					r.Spec.Rabbitmq.AdditionalConfig = "test_config=0"
				}
				if testCase == "advancedConfig" {
					r.Spec.Rabbitmq.AdvancedConfig = "sample-advanced-config."
				}
				if testCase == "envConfig" {
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

			// delete rmq cluster
			Expect(client.Delete(ctx, cluster)).To(Succeed())
			waitForClusterDeletion(ctx, cluster, client)
		},
		EntryDescription("spec.rabbitmq.%s is updated"),
		Entry(nil, "additionalConfig"),
		Entry(nil, "advancedConfig"),
		Entry(nil, "envConfig"),
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
				r.Spec.Replicas = ptr.To(int32(5))
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
