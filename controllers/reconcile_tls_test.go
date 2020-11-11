package controllers_test

import (
	"context"
	"fmt"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Reconcile TLS", func() {
	var (
		cluster          *rabbitmqv1beta1.RabbitmqCluster
		defaultNamespace = "default"
		ctx              = context.Background()
	)
	Context("Mutual TLS", func() {
		AfterEach(func() {
			Expect(client.Delete(ctx, cluster)).To(Succeed())
			Eventually(func() bool {
				rmq := &rabbitmqv1beta1.RabbitmqCluster{}
				err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
				return apierrors.IsNotFound(err)
			}, 5).Should(BeTrue())
		})

		Context("Mutual TLS with single secret", func() {
			It("Deploys successfully", func() {
				tlsSecretWithCACert(ctx, "tls-secret", defaultNamespace)
				tlsSpec := rabbitmqv1beta1.TLSSpec{
					SecretName:   "tls-secret",
					CaSecretName: "tls-secret",
				}
				cluster = rabbitmqClusterWithTLS(ctx, "mutual-tls-success", defaultNamespace, tlsSpec)
				waitForClusterCreation(ctx, cluster, client)

				sts, err := clientSet.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, cluster.ChildResourceName("server"), metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				volumeMount := corev1.VolumeMount{
					Name:      "rabbitmq-tls",
					MountPath: "/etc/rabbitmq-tls/ca.crt",
					SubPath:   "ca.crt",
					ReadOnly:  true,
				}
				Expect(sts.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElement(volumeMount))
			})

			It("Does not deploy if the cert name does not match the contents of the secret", func() {
				tlsData := map[string]string{
					"tls.crt":   "this is a tls cert",
					"tls.key":   "this is a tls key",
					"wrong-key": "certificate",
				}

				_, err := createSecret(ctx, "tls-secret-missing", defaultNamespace, tlsData)

				if !apierrors.IsAlreadyExists(err) {
					Expect(err).NotTo(HaveOccurred())
				}

				tlsSpec := rabbitmqv1beta1.TLSSpec{
					SecretName:   "tls-secret-missing",
					CaSecretName: "tls-secret-missing",
				}
				cluster = rabbitmqClusterWithTLS(ctx, "tls-secret-missing", defaultNamespace, tlsSpec)

				verifyError(ctx, cluster, fmt.Sprintf("The TLS secret tls-secret-missing in namespace %s must have the field ca.crt", defaultNamespace))
			})
		})

		Context("Mutual TLS with a seperate CA certificate secret", func() {
			It("Does not deploy the RabbitmqCluster, and retries every 10 seconds", func() {
				tlsSecretWithoutCACert(ctx, "rabbitmq-tls-secret-does-not-exist", defaultNamespace)

				tlsSpec := rabbitmqv1beta1.TLSSpec{
					SecretName:   "rabbitmq-tls-secret-does-not-exist",
					CaSecretName: "ca-cert-secret",
				}
				cluster = rabbitmqClusterWithTLS(ctx, "rabbitmq-tls-secret-does-not-exist", defaultNamespace, tlsSpec)
				verifyError(ctx, cluster, "Failed to get CA certificate secret")

				_, err := clientSet.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, cluster.ChildResourceName("server"), metav1.GetOptions{})
				Expect(err).To(HaveOccurred())

				// create missing secret
				caData := map[string]string{
					"ca.crt": "this is a ca cert",
				}
				_, err = createSecret(ctx, "ca-cert-secret", defaultNamespace, caData)
				Expect(err).NotTo(HaveOccurred())

				waitForClusterCreation(ctx, cluster, client)
				statefulSet(ctx, cluster)
			})

			It("Does not deploy if the cert name does not match the contents of the secret", func() {
				tlsSecretWithoutCACert(ctx, "tls-secret", defaultNamespace)
				caData := map[string]string{
					"cacrt": "this is a ca cert",
				}

				_, err := createSecret(ctx, "ca-cert-secret-invalid", defaultNamespace, caData)
				if !apierrors.IsAlreadyExists(err) {
					Expect(err).NotTo(HaveOccurred())
				}

				tlsSpec := rabbitmqv1beta1.TLSSpec{
					SecretName:   "tls-secret",
					CaSecretName: "ca-cert-secret-invalid",
				}
				cluster = rabbitmqClusterWithTLS(ctx, "rabbitmq-mutual-tls-missing", defaultNamespace, tlsSpec)
				verifyError(ctx, cluster, fmt.Sprintf("The TLS secret ca-cert-secret-invalid in namespace %s must have the field ca.crt", defaultNamespace))
			})
		})
	})

	Context("TLS set on the instance", func() {
		AfterEach(func() {
			Expect(client.Delete(ctx, cluster)).To(Succeed())
			Eventually(func() bool {
				rmq := &rabbitmqv1beta1.RabbitmqCluster{}
				err := client.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, rmq)
				return apierrors.IsNotFound(err)
			}, 5).Should(BeTrue())
		})

		BeforeEach(func() {
			tlsSecretWithoutCACert(ctx, "tls-secret", defaultNamespace)
		})

		It("Deploys successfully", func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-tls",
					Namespace: defaultNamespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					TLS: rabbitmqv1beta1.TLSSpec{
						SecretName: "tls-secret",
					},
				},
			}

			Expect(client.Create(ctx, cluster)).To(Succeed())
			waitForClusterCreation(ctx, cluster, client)
		})

		When("the TLS secret does not have the expected keys - tls.crt, or tls.key", func() {
			BeforeEach(func() {
				secretData := map[string]string{
					"somekey": "someval",
					"tls.key": "this is a tls key",
				}
				_, err := createSecret(ctx, "rabbitmq-tls-malformed", defaultNamespace, secretData)

				if !apierrors.IsAlreadyExists(err) {
					Expect(err).NotTo(HaveOccurred())
				}

				tlsSpec := rabbitmqv1beta1.TLSSpec{
					SecretName: "rabbitmq-tls-malformed",
				}
				cluster = rabbitmqClusterWithTLS(ctx, "rabbitmq-tls-malformed", defaultNamespace, tlsSpec)
			})

			It("fails to deploy the RabbitmqCluster", func() {
				verifyError(ctx, cluster, fmt.Sprintf("The TLS secret rabbitmq-tls-malformed in namespace %s must have the fields tls.crt and tls.key", defaultNamespace))
			})
		})

		When("the TLS secret does not exist", func() {
			It("fails to deploy the RabbitmqCluster until the secret is detected", func() {

				tlsSpec := rabbitmqv1beta1.TLSSpec{
					SecretName: "tls-secret-does-not-exist",
				}
				cluster = rabbitmqClusterWithTLS(ctx, "rabbitmq-tls-secret-does-not-exist", defaultNamespace, tlsSpec)

				verifyError(ctx, cluster, "Failed to get TLS secret")

				_, err := clientSet.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, cluster.ChildResourceName("server"), metav1.GetOptions{})
				Expect(err).To(HaveOccurred())

				// create missing secret
				secretData := map[string]string{
					"tls.crt": "this is a tls cert",
					"tls.key": "this is a tls key",
				}
				_, err = createSecret(ctx, "tls-secret-does-not-exist", defaultNamespace, secretData)
				Expect(err).NotTo(HaveOccurred())

				waitForClusterCreation(ctx, cluster, client)
				statefulSet(ctx, cluster)
			})
		})
	})
})

func tlsSecretWithCACert(ctx context.Context, secretName, namespace string) {
	tlsData := map[string]string{
		"tls.crt": "this is a tls cert",
		"tls.key": "this is a tls key",
		"ca.crt":  "certificate",
	}

	_, err := createSecret(ctx, secretName, namespace, tlsData)

	if !apierrors.IsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred())
	}
}

func tlsSecretWithoutCACert(ctx context.Context, secretName, namespace string) {
	tlsData := map[string]string{
		"tls.crt": "this is a tls cert",
		"tls.key": "this is a tls key",
	}
	_, err := createSecret(ctx, secretName, namespace, tlsData)

	if !apierrors.IsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred())
	}
}

func rabbitmqClusterWithTLS(ctx context.Context, clustername string, namespace string, tlsSpec rabbitmqv1beta1.TLSSpec) *rabbitmqv1beta1.RabbitmqCluster {
	rabbitmqCluster := &rabbitmqv1beta1.RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clustername,
			Namespace: namespace,
		},
		Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
			TLS: tlsSpec,
		},
	}

	Expect(client.Create(ctx, rabbitmqCluster)).To(Succeed())

	return rabbitmqCluster
}
