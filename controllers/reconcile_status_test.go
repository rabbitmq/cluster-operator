package controllers_test

import (
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/resource"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Reconcile status", func() {
	var (
		cluster          *rabbitmqv1beta1.RabbitmqCluster
		defaultNamespace = "default"
	)

	AfterEach(func() {
		Expect(client.Delete(ctx, cluster)).To(Succeed())
		Eventually(func() bool {
			rmq := &rabbitmqv1beta1.RabbitmqCluster{}
			err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
			return apierrors.IsNotFound(err)
		}, 5).Should(BeTrue())
	})

	It("reconciles the custom resource status", func() {
		cluster = &rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rabbitmq-status",
				Namespace: defaultNamespace,
			},
		}
		Expect(client.Create(ctx, cluster)).To(Succeed())
		waitForClusterCreation(ctx, cluster, client)

		By("setting the default-user secret details")
		rmq := &rabbitmqv1beta1.RabbitmqCluster{}
		secretRef := &rabbitmqv1beta1.RabbitmqClusterSecretReference{}
		Eventually(func() *rabbitmqv1beta1.RabbitmqClusterSecretReference {
			err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
			if err != nil {
				return nil
			}

			if rmq.Status.DefaultUser != nil && rmq.Status.DefaultUser.SecretReference != nil {
				secretRef = rmq.Status.DefaultUser.SecretReference
				return secretRef
			}

			return nil
		}, 5).ShouldNot(BeNil())

		Expect(secretRef.Name).To(Equal(rmq.ChildResourceName(resource.DefaultUserSecretName)))
		Expect(secretRef.Namespace).To(Equal(rmq.Namespace))
		Expect(secretRef.Keys).To(HaveKeyWithValue("username", "username"))
		Expect(secretRef.Keys).To(HaveKeyWithValue("password", "password"))

		By("setting the service details")
		rmq = &rabbitmqv1beta1.RabbitmqCluster{}
		serviceRef := &rabbitmqv1beta1.RabbitmqClusterServiceReference{}
		Eventually(func() *rabbitmqv1beta1.RabbitmqClusterServiceReference {
			err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
			if err != nil {
				return nil
			}

			if rmq.Status.DefaultUser != nil && rmq.Status.DefaultUser.ServiceReference != nil {
				serviceRef = rmq.Status.DefaultUser.ServiceReference
				return serviceRef
			}

			return nil
		}, 5).ShouldNot(BeNil())

		Expect(serviceRef.Name).To(Equal(rmq.ChildResourceName("")))
		Expect(serviceRef.Namespace).To(Equal(rmq.Namespace))

		By("setting Status.Binding")
		rmq = &rabbitmqv1beta1.RabbitmqCluster{}
		binding := &corev1.LocalObjectReference{}
		Eventually(func() *corev1.LocalObjectReference {
			Expect(client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)).To(Succeed())
			if rmq.Status.Binding != nil {
				binding = rmq.Status.Binding
				return binding
			}
			return nil
		}, 5).ShouldNot(BeNil())

		Expect(binding.Name).To(Equal(rmq.ChildResourceName(resource.DefaultUserSecretName)))
	})

	When("secret backend vault is enabled", func() {
		It("sets service reference status correctly", func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-vault-status",
					Namespace: defaultNamespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					SecretBackend: rabbitmqv1beta1.SecretBackend{
						Vault: &rabbitmqv1beta1.VaultSpec{
							Role:            "rabbit",
							DefaultUserPath: "test-test",
						},
					},
				},
			}
			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)

			By("setting the service details")
			rmq := &rabbitmqv1beta1.RabbitmqCluster{}
			serviceRef := &rabbitmqv1beta1.RabbitmqClusterServiceReference{}
			Eventually(func() *rabbitmqv1beta1.RabbitmqClusterServiceReference {
				err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
				if err != nil {
					return nil
				}
				if rmq.Status.DefaultUser != nil && rmq.Status.DefaultUser.ServiceReference != nil {
					serviceRef = rmq.Status.DefaultUser.ServiceReference
					return serviceRef
				}
				return nil
			}, 5).ShouldNot(BeNil())

			Expect(serviceRef.Name).To(Equal(rmq.ChildResourceName("")))
			Expect(serviceRef.Namespace).To(Equal(rmq.Namespace))

			By("leaving status.binding and secret reference empty")
			rmq = &rabbitmqv1beta1.RabbitmqCluster{}
			Consistently(func() *corev1.LocalObjectReference {
				Expect(client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)).To(Succeed())
				return rmq.Status.Binding
			}).Should(BeNil())

			Consistently(func() *rabbitmqv1beta1.RabbitmqClusterSecretReference {
				Expect(client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)).To(Succeed())
				return rmq.Status.DefaultUser.SecretReference
			}).Should(BeNil())
		})
	})

})
