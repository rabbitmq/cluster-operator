package controllers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("ReconcileOperatorDefaults", func() {
	var cluster *rabbitmqv1beta1.RabbitmqCluster

	BeforeEach(func() {
		cluster = &rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-default",
				Namespace: "default",
			},
			Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
				SecretBackend: rabbitmqv1beta1.SecretBackend{
					Vault: &rabbitmqv1beta1.VaultSpec{
						DefaultUserPath: "some-path",
					},
				},
			},
		}

		Expect(client.Create(ctx, cluster)).To(Succeed())
		waitForClusterCreation(ctx, cluster, client)
	})

	AfterEach(func() {
		Expect(client.Delete(ctx, cluster)).To(Succeed())
	})

	It("handles operator defaults correctly", func() {
		fetchedCluster := &rabbitmqv1beta1.RabbitmqCluster{}
		Expect(client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, fetchedCluster)).To(Succeed())

		By("setting the image spec with the default image")
		Expect(fetchedCluster.Spec.Image).To(Equal(defaultRabbitmqImage))

		By("setting the default user updater image to the controller default")
		Expect(fetchedCluster.Spec.SecretBackend.Vault.DefaultUserUpdaterImage).To(PointTo(Equal(defaultUserUpdaterImage)))

		By("setting the default imagePullSecrets")
		Expect(statefulSet(ctx, cluster).Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(
			[]corev1.LocalObjectReference{
				{
					Name: "image-secret-1",
				},
				{
					Name: "image-secret-2",
				},
				{
					Name: "image-secret-3",
				},
			},
		))
	})
})
