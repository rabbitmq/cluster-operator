package topologycontrollers_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("federation-controller", func() {
	var federation rabbitmqv1beta1.Federation
	var federationName string

	JustBeforeEach(func() {
		federation = rabbitmqv1beta1.Federation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      federationName,
				Namespace: "default",
			},
			Spec: rabbitmqv1beta1.FederationSpec{
				Name:      "my-federation-upstream",
				Vhost:     "/test",
				UriSecret: &corev1.LocalObjectReference{Name: "federation-uri"},
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: "example-rabbit",
				},
			},
		}
	})

	When("creation", func() {
		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				federationName = "test-federation-http-error"
				fakeRabbitMQClient.PutFederationUpstreamReturns(&http.Response{
					Status:     "418 I'm a teapot",
					StatusCode: 418,
				}, errors.New("some HTTP error"))
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(client.Create(ctx, &federation)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace},
						&federation,
					)

					return federation.Status.Conditions
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
				federationName = "test-federation-go-error"
				fakeRabbitMQClient.PutFederationUpstreamReturns(nil, errors.New("some go failure here"))
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(client.Create(ctx, &federation)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace},
						&federation,
					)

					return federation.Status.Conditions
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
				federationName = "test-federation-success"
				fakeRabbitMQClient.PutFederationUpstreamReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
			})

			It("works", func() {
				Expect(client.Create(ctx, &federation)).To(Succeed())
				By("setting the correct finalizer")
				Eventually(func() []string {
					var fetched rabbitmqv1beta1.Federation
					err := client.Get(ctx, types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace}, &fetched)
					if err != nil {
						return []string{}
					}
					return fetched.ObjectMeta.Finalizers
				}, 5).Should(ConsistOf("deletion.finalizers.federations.rabbitmq.com"))

				By("sets the status condition 'Ready' to 'true'")
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace},
						&federation,
					)

					return federation.Status.Conditions
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
			fakeRabbitMQClient.PutFederationUpstreamReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			Expect(client.Create(ctx, &federation)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace},
					&federation,
				)

				return federation.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				federationName = "delete-federation-http-error"
				fakeRabbitMQClient.DeleteFederationUpstreamReturns(&http.Response{
					Status:     "502 Bad Gateway",
					StatusCode: http.StatusBadGateway,
					Body:       ioutil.NopCloser(bytes.NewBufferString("Hello World")),
				}, nil)
			})

			It("raises an event to indicate a failure to delete", func() {
				Expect(client.Delete(ctx, &federation)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace}, &rabbitmqv1beta1.Federation{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete federation"))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				federationName = "delete-federation-go-error"
				fakeRabbitMQClient.DeleteFederationUpstreamReturns(nil, errors.New("some error"))
			})

			It("publishes a 'warning' event", func() {
				Expect(client.Delete(ctx, &federation)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace}, &rabbitmqv1beta1.Federation{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete federation"))
			})
		})

		Context("success", func() {
			BeforeEach(func() {
				federationName = "delete-federation-success"
				fakeRabbitMQClient.DeleteFederationUpstreamReturns(&http.Response{
					Status:     "204 No Content",
					StatusCode: http.StatusNoContent,
				}, nil)
			})

			It("publishes a normal event", func() {
				Expect(client.Delete(ctx, &federation)).To(Succeed())
				Eventually(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace}, &rabbitmqv1beta1.Federation{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeTrue())
				Expect(observedEvents()).To(SatisfyAll(
					Not(ContainElement("Warning FailedDelete failed to delete federation")),
					ContainElement("Normal SuccessfulDelete successfully deleted federation"),
				))
			})
		})
	})

	When("a federation references a cluster from a prohibited namespace", func() {
		JustBeforeEach(func() {
			federationName = "test-federation-prohibited"
			federation = rabbitmqv1beta1.Federation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      federationName,
					Namespace: "prohibited",
				},
				Spec: rabbitmqv1beta1.FederationSpec{
					Name:      "my-federation-upstream",
					Vhost:     "/test",
					UriSecret: &corev1.LocalObjectReference{Name: "federation-uri"},
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
		})
		It("should throw an error about a cluster being prohibited", func() {
			Expect(client.Create(ctx, &federation)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace},
					&federation,
				)

				return federation.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason":  Equal("FailedCreateOrUpdate"),
				"Status":  Equal(corev1.ConditionFalse),
				"Message": ContainSubstring("not allowed to reference"),
			})))
		})
	})

	When("a federation references a cluster from an allowed namespace", func() {
		JustBeforeEach(func() {
			federationName = "test-federation-allowed"
			federation = rabbitmqv1beta1.Federation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      federationName,
					Namespace: "allowed",
				},
				Spec: rabbitmqv1beta1.FederationSpec{
					Name:      "my-federation-upstream",
					Vhost:     "/test",
					UriSecret: &corev1.LocalObjectReference{Name: "federation-uri"},
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
			fakeRabbitMQClient.PutFederationUpstreamReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &federation)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace},
					&federation,
				)

				return federation.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})

	When("a federation references a cluster that allows all namespaces", func() {
		JustBeforeEach(func() {
			federationName = "test-federation-allowed-when-allow-all"
			federation = rabbitmqv1beta1.Federation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      federationName,
					Namespace: "prohibited",
				},
				Spec: rabbitmqv1beta1.FederationSpec{
					Name:      "my-federation-upstream",
					Vhost:     "/test",
					UriSecret: &corev1.LocalObjectReference{Name: "federation-uri"},
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "allow-all-rabbit",
						Namespace: "default",
					},
				},
			}
			fakeRabbitMQClient.PutFederationUpstreamReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &federation)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace},
					&federation,
				)

				return federation.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})
})
