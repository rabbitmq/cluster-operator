package topologycontrollers_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
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

var _ = Describe("exchange-controller", func() {
	var exchange rabbitmqv1beta1.Exchange
	var exchangeName string

	JustBeforeEach(func() {
		exchange = rabbitmqv1beta1.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      exchangeName,
				Namespace: "default",
			},
			Spec: rabbitmqv1beta1.ExchangeSpec{
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: "example-rabbit",
				},
			},
		}
	})

	Context("creation", func() {
		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				exchangeName = "test-http-error"
				fakeRabbitMQClient.DeclareExchangeReturns(&http.Response{
					Status:     "418 I'm a teapot",
					StatusCode: 418,
				}, errors.New("a failure"))
			})

			It("sets the status condition", func() {
				Expect(client.Create(ctx, &exchange)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace},
						&exchange,
					)

					return exchange.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(rabbitmqv1beta1.ConditionType("Ready")),
					"Reason":  Equal("FailedCreateOrUpdate"),
					"Status":  Equal(corev1.ConditionFalse),
					"Message": ContainSubstring("a failure"),
				})))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				exchangeName = "test-go-error"
				fakeRabbitMQClient.DeclareExchangeReturns(nil, errors.New("a go failure"))
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(client.Create(ctx, &exchange)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace},
						&exchange,
					)

					return exchange.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(rabbitmqv1beta1.ConditionType("Ready")),
					"Reason":  Equal("FailedCreateOrUpdate"),
					"Status":  Equal(corev1.ConditionFalse),
					"Message": ContainSubstring("a go failure"),
				})))
			})
		})

		When("success", func() {
			BeforeEach(func() {
				exchangeName = "test-create-success"
				fakeRabbitMQClient.DeclareExchangeReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
			})

			It("works", func() {
				Expect(client.Create(ctx, &exchange)).To(Succeed())
				By("setting the correct finalizer")
				Eventually(komega.Object(&exchange)).WithTimeout(2 * time.Second).Should(HaveField("ObjectMeta.Finalizers", ConsistOf("deletion.finalizers.exchanges.rabbitmq.com")))

				By("sets the status condition 'Ready' to 'true'")
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace},
						&exchange,
					)

					return exchange.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})
		})
	})
	Context("LastTransitionTime", func() {
		BeforeEach(func() {
			exchangeName = "test-last-transition-time"
			fakeRabbitMQClient.DeclareExchangeReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("changes only if status changes", func() {
			By("setting LastTransitionTime when transitioning to status Ready=true")
			Expect(client.Create(ctx, &exchange)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Namespace: exchange.Namespace, Name: exchange.Name},
					&exchange,
				)
				return exchange.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ConsistOf(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Status": Equal(corev1.ConditionTrue),
			})))
			lastTransitionTime := exchange.Status.Conditions[0].LastTransitionTime
			Expect(lastTransitionTime.IsZero()).To(BeFalse())

			By("not touching LastTransitionTime when staying in status Ready=true")
			fakeRabbitMQClient.DeclareExchangeReturns(&http.Response{
				Status:     "204 No Content",
				StatusCode: http.StatusNoContent,
			}, nil)
			exchange.Labels = map[string]string{"k1": "v1"}
			Expect(client.Update(ctx, &exchange)).To(Succeed())
			ConsistentlyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Namespace: exchange.Namespace, Name: exchange.Name},
					&exchange,
				)
				return exchange.Status.Conditions
			}, "3s").Should(ConsistOf(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Status": Equal(corev1.ConditionTrue),
			})))
			Expect(exchange.Status.Conditions[0].LastTransitionTime.Time).To(BeTemporally("==", lastTransitionTime.Time))

			By("updating LastTransitionTime when transitioning to status Ready=false")
			fakeRabbitMQClient.DeclareExchangeReturns(&http.Response{
				Status:     "500 Internal Server Error",
				StatusCode: http.StatusInternalServerError,
			}, errors.New("something went wrong"))
			exchange.Labels = map[string]string{"k1": "v2"}
			Expect(client.Update(ctx, &exchange)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Namespace: exchange.Namespace, Name: exchange.Name},
					&exchange,
				)
				return exchange.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ConsistOf(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Status":  Equal(corev1.ConditionFalse),
				"Reason":  Equal("FailedCreateOrUpdate"),
				"Message": Equal("something went wrong"),
			})))
			Expect(exchange.Status.Conditions[0].LastTransitionTime.Time).To(BeTemporally(">", lastTransitionTime.Time))
		})
	})

	Context("deletion", func() {
		JustBeforeEach(func() {
			fakeRabbitMQClient.DeclareExchangeReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			Expect(client.Create(ctx, &exchange)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace},
					&exchange,
				)

				return exchange.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				exchangeName = "delete-exchange-http-error"
				fakeRabbitMQClient.DeleteExchangeReturns(&http.Response{
					Status:     "502 Bad Gateway",
					StatusCode: http.StatusBadGateway,
					Body:       ioutil.NopCloser(bytes.NewBufferString("Hello World")),
				}, nil)
			})

			It("publishes a 'warning' event", func() {
				Expect(client.Delete(ctx, &exchange)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace}, &rabbitmqv1beta1.Exchange{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete exchange"))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				exchangeName = "delete-go-error"
				fakeRabbitMQClient.DeleteExchangeReturns(nil, errors.New("some error"))
			})

			It("publishes a 'warning' event", func() {
				Expect(client.Delete(ctx, &exchange)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace}, &rabbitmqv1beta1.Exchange{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete exchange"))
			})
		})

		When("the RabbitMQ Client successfully deletes a exchange", func() {
			BeforeEach(func() {
				exchangeName = "delete-exchange-success"
				fakeRabbitMQClient.DeleteExchangeReturns(&http.Response{
					Status:     "204 No Content",
					StatusCode: http.StatusNoContent,
				}, nil)
			})

			It("publishes a normal event", func() {
				Expect(client.Delete(ctx, &exchange)).To(Succeed())
				Eventually(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace}, &rabbitmqv1beta1.Exchange{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeTrue())
				Expect(observedEvents()).To(SatisfyAll(
					Not(ContainElement("Warning FailedDelete failed to delete exchange")),
					ContainElement("Normal SuccessfulDelete successfully deleted exchange"),
				))
			})
		})
	})

	When("an exchange references a cluster from a prohibited namespace", func() {
		JustBeforeEach(func() {
			exchangeName = "test-exchange-prohibited"
			exchange = rabbitmqv1beta1.Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      exchangeName,
					Namespace: "prohibited",
				},
				Spec: rabbitmqv1beta1.ExchangeSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
		})
		It("should throw an error about a cluster being prohibited", func() {
			Expect(client.Create(ctx, &exchange)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace},
					&exchange,
				)

				return exchange.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason":  Equal("FailedCreateOrUpdate"),
				"Status":  Equal(corev1.ConditionFalse),
				"Message": ContainSubstring("not allowed to reference"),
			})))
		})
	})

	When("an exchange references a cluster from an allowed namespace", func() {
		JustBeforeEach(func() {
			exchangeName = "test-exchange-allowed"
			exchange = rabbitmqv1beta1.Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      exchangeName,
					Namespace: "allowed",
				},
				Spec: rabbitmqv1beta1.ExchangeSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
			fakeRabbitMQClient.DeclareExchangeReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &exchange)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace},
					&exchange,
				)

				return exchange.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})

	When("an exchange references a cluster that allows all namespaces", func() {
		JustBeforeEach(func() {
			exchangeName = "test-exchange-allowed-when-allow-all"
			exchange = rabbitmqv1beta1.Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      exchangeName,
					Namespace: "prohibited",
				},
				Spec: rabbitmqv1beta1.ExchangeSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "allow-all-rabbit",
						Namespace: "default",
					},
				},
			}
			fakeRabbitMQClient.DeclareExchangeReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &exchange)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace},
					&exchange,
				)

				return exchange.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})
})
