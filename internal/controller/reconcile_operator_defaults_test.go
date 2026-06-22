package controllers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ReconcileOperatorDefaults", func() {
	Context("with defaults already applied (as the mutating webhook would)", func() {
		var cluster *rabbitmqv1beta1.RabbitmqCluster

		BeforeEach(func() {
			// Simulate what the mutating webhook sets at admission time.
			userUpdaterImage := defaultUserUpdaterImage
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-default",
					Namespace: "default",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Image: defaultRabbitmqImage,
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "image-secret-1"},
						{Name: "image-secret-2"},
						{Name: "image-secret-3"},
					},
					SecretBackend: rabbitmqv1beta1.SecretBackend{
						Vault: &rabbitmqv1beta1.VaultSpec{
							DefaultUserPath:         "some-path",
							DefaultUserUpdaterImage: &userUpdaterImage,
						},
					},
				},
			}

			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)
		})

		It("propagates defaults to child resources", func() {
			fetchedCluster := &rabbitmqv1beta1.RabbitmqCluster{}
			Expect(client.Get(ctx, k8sclient.ObjectKeyFromObject(cluster), fetchedCluster)).To(Succeed())

			By("preserving the image set by the webhook")
			Expect(fetchedCluster.Spec.Image).To(Equal(defaultRabbitmqImage))

			By("preserving the default user updater image set by the webhook")
			Expect(fetchedCluster.Spec.SecretBackend.Vault.DefaultUserUpdaterImage).To(PointTo(Equal(defaultUserUpdaterImage)))

			By("propagating imagePullSecrets to the StatefulSet")
			Expect(statefulSet(ctx, cluster).Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(
				[]corev1.LocalObjectReference{
					{Name: "image-secret-1"},
					{Name: "image-secret-2"},
					{Name: "image-secret-3"},
				},
			))
		})
	})
})
