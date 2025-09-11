package controllers_test

import (
	"context"
	"fmt"
	"time"

	"github.com/rabbitmq/cluster-operator/v2/internal/status"
	"k8s.io/utils/ptr"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Reconcile TLS", func() {
	var (
		cluster          *rabbitmqv1beta1.RabbitmqCluster
		defaultNamespace = "default"
		ctx              = context.Background()
	)
	Context("Mutual TLS", func() {
		Context("Mutual TLS with single secret", func() {
			const tlsSecretName = "tls-secret-success"

			It("Deploys successfully", func() {
				tlsSecretWithCACert(ctx, tlsSecretName, defaultNamespace)
				tlsSpec := rabbitmqv1beta1.TLSSpec{
					SecretName:   tlsSecretName,
					CaSecretName: tlsSecretName,
				}
				cluster = rabbitmqClusterWithTLS(ctx, "mutual-tls-success", defaultNamespace, tlsSpec)
				waitForClusterCreation(ctx, cluster, client)

				sts, err := clientSet.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, cluster.ChildResourceName("server"), metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				Expect(sts.Spec.Template.Spec.Volumes).To(ContainElement(corev1.Volume{
					Name: "rabbitmq-tls",
					VolumeSource: corev1.VolumeSource{
						Projected: &corev1.ProjectedVolumeSource{
							Sources: []corev1.VolumeProjection{
								{
									Secret: &corev1.SecretProjection{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: tlsSecretName,
										},
										Optional: ptr.To(true),
										Items: []corev1.KeyToPath{
											{Key: "tls.crt", Path: "tls.crt"},
											{Key: "tls.key", Path: "tls.key"},
										},
									},
								},
								{
									Secret: &corev1.SecretProjection{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: tlsSecretName,
										},
										Optional: ptr.To(true),
										Items:    []corev1.KeyToPath{{Key: "ca.crt", Path: "ca.crt"}},
									},
								},
							},
							DefaultMode: ptr.To(int32(400)),
						},
					},
				}))

				Expect(extractContainer(sts.Spec.Template.Spec.Containers, "rabbitmq").VolumeMounts).To(ContainElement(corev1.VolumeMount{
					Name:      "rabbitmq-tls",
					MountPath: "/etc/rabbitmq-tls/",
					ReadOnly:  true,
				}))
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

				verifyTLSErrorEvents(ctx, cluster, fmt.Sprintf("TLS secret tls-secret-missing in namespace %s does not have the field ca.crt", defaultNamespace))
				verifyReconcileSuccessFalse(cluster.Name, cluster.Namespace)
			})
		})

		Context("Mutual TLS with a separate CA certificate secret", func() {
			It("Does not deploy the RabbitmqCluster, and retries every 10 seconds", func() {
				tlsSecretWithoutCACert(ctx, "rabbitmq-tls-secret-initially-missing", defaultNamespace)

				tlsSpec := rabbitmqv1beta1.TLSSpec{
					SecretName:   "rabbitmq-tls-secret-initially-missing",
					CaSecretName: "ca-cert-secret-initially-missing",
				}
				cluster = rabbitmqClusterWithTLS(ctx, "rabbitmq-tls-secret-initially-missing", defaultNamespace, tlsSpec)
				verifyTLSErrorEvents(ctx, cluster, "Failed to get CA certificate secret")
				verifyReconcileSuccessFalse(cluster.Name, cluster.Namespace)

				_, err := clientSet.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, cluster.ChildResourceName("server"), metav1.GetOptions{})
				Expect(err).To(HaveOccurred())

				// create missing secret
				caData := map[string]string{
					"ca.crt": "this is a ca cert",
				}
				_, err = createSecret(ctx, "ca-cert-secret-initially-missing", defaultNamespace, caData)
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
				verifyTLSErrorEvents(ctx, cluster, fmt.Sprintf("TLS secret ca-cert-secret-invalid in namespace %s does not have the field ca.crt", defaultNamespace))
				verifyReconcileSuccessFalse(cluster.Name, cluster.Namespace)
			})
		})
	})

	Context("TLS set on the instance", func() {
		BeforeEach(func() {
			tlsSecretWithoutCACert(ctx, "tls-secret", defaultNamespace)
		})

		It("Deploys successfully", func() {
			suffix := fmt.Sprintf("-%d", time.Now().UnixNano())
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbitmq-tls" + suffix,
					Namespace: defaultNamespace,
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					TLS: rabbitmqv1beta1.TLSSpec{
						SecretName: "tls-secret" + suffix,
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
				verifyTLSErrorEvents(ctx, cluster, fmt.Sprintf("TLS secret rabbitmq-tls-malformed in namespace %s does not have the fields tls.crt and tls.key", defaultNamespace))
				verifyReconcileSuccessFalse(cluster.Name, cluster.Namespace)
			})
		})

		When("the TLS secret does not exist", func() {
			It("fails to deploy the RabbitmqCluster until the secret is detected", func() {
				tlsSpec := rabbitmqv1beta1.TLSSpec{
					SecretName: "tls-secret-does-not-exist",
				}
				suffix := fmt.Sprintf("-%d", time.Now().UnixNano())
				cluster = rabbitmqClusterWithTLS(ctx, "rabbitmq-tls-secret-does-not-exist"+suffix, defaultNamespace, tlsSpec)

				verifyTLSErrorEvents(ctx, cluster, "Failed to get TLS secret")
				verifyReconcileSuccessFalse(cluster.Name, cluster.Namespace)

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

	When("DiableNonTLSListeners set to true", func() {
		It("logs TLSError and set ReconcileSuccess to false when TLS is not enabled", func() {
			tlsSpec := rabbitmqv1beta1.TLSSpec{
				DisableNonTLSListeners: true,
			}
			cluster = rabbitmqClusterWithTLS(ctx, "rabbitmq-disablenontlslisteners", defaultNamespace, tlsSpec)

			verifyTLSErrorEvents(ctx, cluster, "TLS must be enabled if disableNonTLSListeners is set to true")

			_, err := clientSet.AppsV1().StatefulSets(cluster.Namespace).Get(ctx, cluster.ChildResourceName("server"), metav1.GetOptions{})
			Expect(err).To(HaveOccurred())
			verifyReconcileSuccessFalse(cluster.Name, cluster.Namespace)
		})
	})
})

func verifyReconcileSuccessFalse(name, namespace string) bool {
	return EventuallyWithOffset(1, func() string {
		rabbit := &rabbitmqv1beta1.RabbitmqCluster{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		Expect(client.Get(ctx, runtimeClient.ObjectKeyFromObject(rabbit), rabbit)).To(Succeed())

		for i := range rabbit.Status.Conditions {
			if rabbit.Status.Conditions[i].Type == status.ReconcileSuccess {
				return fmt.Sprintf("ReconcileSuccess status: %s", rabbit.Status.Conditions[i].Status)
			}
		}
		return "ReconcileSuccess status: condition not present"
	}, 5).Should(Equal("ReconcileSuccess status: False"))
}

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
