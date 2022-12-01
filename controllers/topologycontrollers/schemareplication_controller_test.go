package topologycontrollers_test

import (
	"bytes"
	"errors"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/controllers/topologycontrollers"
	"github.com/rabbitmq/cluster-operator/internal/rabbitmqclient"
	"github.com/rabbitmq/cluster-operator/internal/rabbitmqclient/rabbitmqclientfakes"
	"github.com/rabbitmq/cluster-operator/internal/topology"
	"io/ioutil"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("schema-replication-controller", func() {
	var replication rabbitmqv1beta1.SchemaReplication
	var replicationName string

	JustBeforeEach(func() {
		replication = rabbitmqv1beta1.SchemaReplication{
			ObjectMeta: metav1.ObjectMeta{
				Name:      replicationName,
				Namespace: "default",
			},
			Spec: rabbitmqv1beta1.SchemaReplicationSpec{
				UpstreamSecret: &corev1.LocalObjectReference{
					Name: "endpoints-secret", // created in 'BeforeSuite'
				},
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: "example-rabbit",
				},
			},
		}
	})

	When("creation", func() {
		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				replicationName = "test-replication-http-error"
				fakeRabbitMQClient.PutGlobalParameterReturns(&http.Response{
					Status:     "418 I'm a teapot",
					StatusCode: 418,
				}, errors.New("some HTTP error"))
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(client.Create(ctx, &replication)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace},
						&replication,
					)

					return replication.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(rabbitmqv1beta1.ConditionType("Ready")),
					"Reason":  Equal("FailedCreateOrUpdate"),
					"Status":  Equal(corev1.ConditionFalse),
					"Message": ContainSubstring("some HTTP error"),
				})))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				replicationName = "test-replication-go-error"
				fakeRabbitMQClient.PutGlobalParameterReturns(nil, errors.New("some go failure here"))
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(client.Create(ctx, &replication)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace},
						&replication,
					)

					return replication.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(rabbitmqv1beta1.ConditionType("Ready")),
					"Reason":  Equal("FailedCreateOrUpdate"),
					"Status":  Equal(corev1.ConditionFalse),
					"Message": ContainSubstring("some go failure here"),
				})))
			})
		})

		Context("success", func() {
			BeforeEach(func() {
				replicationName = "test-replication-success"
				fakeRabbitMQClient.PutGlobalParameterReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
			})

			It("works", func() {
				Expect(client.Create(ctx, &replication)).To(Succeed())
				By("setting the correct finalizer")
				Eventually(komega.Object(&replication)).WithTimeout(2 * time.Second).Should(HaveField("ObjectMeta.Finalizers", ConsistOf("deletion.finalizers.schemareplications.rabbitmq.com")))

				By("sets the status condition 'Ready' to 'true'")
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace},
						&replication,
					)

					return replication.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})
		})
	})

	When("deletion", func() {
		JustBeforeEach(func() {
			fakeRabbitMQClient.PutGlobalParameterReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			Expect(client.Create(ctx, &replication)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace},
					&replication,
				)

				return replication.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				replicationName = "delete-replication-http-error"
				fakeRabbitMQClient.DeleteGlobalParameterReturns(&http.Response{
					Status:     "502 Bad Gateway",
					StatusCode: http.StatusBadGateway,
					Body:       ioutil.NopCloser(bytes.NewBufferString("Hello World")),
				}, nil)
			})

			It("raises an event to indicate a failure to delete", func() {
				Expect(client.Delete(ctx, &replication)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace}, &rabbitmqv1beta1.SchemaReplication{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete schemareplication"))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				replicationName = "delete-replication-go-error"
				fakeRabbitMQClient.DeleteGlobalParameterReturns(nil, errors.New("some error"))
			})

			It("publishes a 'warning' event", func() {
				Expect(client.Delete(ctx, &replication)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace}, &rabbitmqv1beta1.SchemaReplication{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete schemareplication"))
			})
		})

		Context("success", func() {
			BeforeEach(func() {
				replicationName = "delete-replication-success"
				fakeRabbitMQClient.DeleteGlobalParameterReturns(&http.Response{
					Status:     "204 No Content",
					StatusCode: http.StatusNoContent,
				}, nil)
			})

			It("publishes a normal event", func() {
				Expect(client.Delete(ctx, &replication)).To(Succeed())
				Eventually(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace}, &rabbitmqv1beta1.SchemaReplication{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeTrue())
				Expect(observedEvents()).To(SatisfyAll(
					Not(ContainElement("Warning FailedDelete failed to deleted schemareplication")),
					ContainElement("Normal SuccessfulDelete successfully deleted schemareplication"),
				))
			})
		})
	})

	When("a schema replication references a cluster from a prohibited namespace", func() {
		JustBeforeEach(func() {
			replicationName = "test-replication-prohibited"
			replication = rabbitmqv1beta1.SchemaReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      replicationName,
					Namespace: "prohibited",
				},
				Spec: rabbitmqv1beta1.SchemaReplicationSpec{
					UpstreamSecret: &corev1.LocalObjectReference{
						Name: "endpoints-secret", // created in 'BeforeSuite'
					},
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
		})
		It("should throw an error about a cluster being prohibited", func() {
			Expect(client.Create(ctx, &replication)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace},
					&replication,
				)

				return replication.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason":  Equal("FailedCreateOrUpdate"),
				"Status":  Equal(corev1.ConditionFalse),
				"Message": ContainSubstring("not allowed to reference"),
			})))
		})
	})

	When("a schema replication references a cluster from an allowed namespace", func() {
		JustBeforeEach(func() {
			replicationName = "test-replication-allowed"
			replication = rabbitmqv1beta1.SchemaReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      replicationName,
					Namespace: "allowed",
				},
				Spec: rabbitmqv1beta1.SchemaReplicationSpec{
					UpstreamSecret: &corev1.LocalObjectReference{
						Name: "endpoints-secret",
					},
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
			fakeRabbitMQClient.PutGlobalParameterReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &replication)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace},
					&replication,
				)

				return replication.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})

	When("a schema replication references a cluster that allows all namespaces", func() {
		JustBeforeEach(func() {
			replicationName = "test-replication-allowed-when-allow-all"
			replication = rabbitmqv1beta1.SchemaReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      replicationName,
					Namespace: "prohibited",
				},
				Spec: rabbitmqv1beta1.SchemaReplicationSpec{
					UpstreamSecret: &corev1.LocalObjectReference{
						Name: "endpoints-secret",
					},
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "allow-all-rabbit",
						Namespace: "default",
					},
				},
			}
			fakeRabbitMQClient.PutGlobalParameterReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &replication)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace},
					&replication,
				)

				return replication.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})

	When("a schema replication uses vault as secretBackend", func() {
		JustBeforeEach(func() {
			replicationName = "vault"
			replication = rabbitmqv1beta1.SchemaReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      replicationName,
					Namespace: "default",
				},
				Spec: rabbitmqv1beta1.SchemaReplicationSpec{
					SecretBackend: rabbitmqv1beta1.SchemaReplicationSecretBackend{Vault: &rabbitmqv1beta1.SchemaReplicationVaultSpec{SecretPath: "rabbitmq"}},
					Endpoints:     "test:12345",
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}

			fakeRabbitMQClient.PutGlobalParameterReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})

		AfterEach(func() {
			rabbitmqclient.SecretStoreClientProvider = rabbitmqclient.GetSecretStoreClient
		})

		It("set schema sync parameters with generated correct endpoints", func() {
			fakeSecretStoreClient := &rabbitmqclientfakes.FakeSecretStoreClient{}
			fakeSecretStoreClient.ReadCredentialsReturns("a-user-in-vault", "test", nil)
			rabbitmqclient.SecretStoreClientProvider = func() (rabbitmqclient.SecretStoreClient, error) {
				return fakeSecretStoreClient, nil
			}

			Expect(client.Create(ctx, &replication)).To(Succeed())
			Eventually(func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace},
					&replication,
				)
				return replication.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))

			parameter, endpoints := fakeRabbitMQClient.PutGlobalParameterArgsForCall(1)
			Expect(parameter).To(Equal(topologycontrollers.SchemaReplicationParameterName))
			Expect(endpoints.(internal.UpstreamEndpoints).Username).To(Equal("a-user-in-vault"))
			Expect(endpoints.(internal.UpstreamEndpoints).Password).To(Equal("test"))
			Expect(endpoints.(internal.UpstreamEndpoints).Endpoints).To(ConsistOf("test:12345"))
		})
	})
})
