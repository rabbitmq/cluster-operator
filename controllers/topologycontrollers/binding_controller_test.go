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

var _ = Describe("bindingController", func() {
	var binding rabbitmqv1beta1.Binding
	var bindingName string

	JustBeforeEach(func() {
		binding = rabbitmqv1beta1.Binding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bindingName,
				Namespace: "default",
			},
			Spec: rabbitmqv1beta1.BindingSpec{
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: "example-rabbit",
				},
			},
		}
	})

	When("creating a binding", func() {
		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				bindingName = "test-binding-http-error"
				fakeRabbitMQClient.DeclareBindingReturns(&http.Response{
					Status:     "418 I'm a teapot",
					StatusCode: 418,
				}, errors.New("some HTTP error"))
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(client.Create(ctx, &binding)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace},
						&binding,
					)

					return binding.Status.Conditions
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
				bindingName = "test-binding-go-error"
				fakeRabbitMQClient.DeclareBindingReturns(nil, errors.New("hit a exception"))
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(client.Create(ctx, &binding)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace},
						&binding,
					)

					return binding.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(rabbitmqv1beta1.ConditionType("Ready")),
					"Reason":  Equal("FailedCreateOrUpdate"),
					"Status":  Equal(corev1.ConditionFalse),
					"Message": ContainSubstring("hit a exception"),
				})))
			})
		})

		Context("success", func() {
			BeforeEach(func() {
				bindingName = "test-binding-success"
				fakeRabbitMQClient.DeclareBindingReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
			})

			It("works", func() {
				Expect(client.Create(ctx, &binding)).To(Succeed())
				By("setting the correct finalizer")
				Eventually(komega.Object(&binding)).WithTimeout(2 * time.Second).Should(HaveField("ObjectMeta.Finalizers", ConsistOf("deletion.finalizers.bindings.rabbitmq.com")))

				By("sets the status condition 'Ready' to 'true'")
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace},
						&binding,
					)

					return binding.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})
		})
	})

	When("Deleting a binding", func() {
		JustBeforeEach(func() {
			fakeRabbitMQClient.DeclareBindingReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			Expect(client.Create(ctx, &binding)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace},
					&binding,
				)

				return binding.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				bindingName = "delete-binding-http-error"
				fakeRabbitMQClient.DeleteBindingReturns(&http.Response{
					Status:     "502 Bad Gateway",
					StatusCode: http.StatusBadGateway,
					Body:       ioutil.NopCloser(bytes.NewBufferString("Hello World")),
				}, nil)
			})

			It("raises an event to indicate a failure to delete", func() {
				Expect(client.Delete(ctx, &binding)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace}, &rabbitmqv1beta1.Binding{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete binding"))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				bindingName = "delete-binding-go-error"
				fakeRabbitMQClient.DeleteBindingReturns(nil, errors.New("some error"))
			})

			It("raises an event to indicate a failure to delete", func() {
				Expect(client.Delete(ctx, &binding)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace}, &rabbitmqv1beta1.Binding{})
					return apierrors.IsNotFound(err)
				}, 5).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete binding"))
			})
		})

		When("the RabbitMQ Client successfully deletes a binding", func() {
			BeforeEach(func() {
				bindingName = "delete-binding-success"
				fakeRabbitMQClient.DeleteBindingReturns(&http.Response{
					Status:     "204 No Content",
					StatusCode: http.StatusNoContent,
				}, nil)
			})

			It("raises an event to indicate a successful deletion", func() {
				Expect(client.Delete(ctx, &binding)).To(Succeed())
				Eventually(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace}, &rabbitmqv1beta1.Binding{})
					return apierrors.IsNotFound(err)
				}, 5).Should(BeTrue())
				Expect(observedEvents()).To(SatisfyAll(
					Not(ContainElement("Warning FailedDelete failed to delete binding")),
					ContainElement("Normal SuccessfulDelete successfully deleted binding"),
				))
			})
		})
	})

	When("a binding references a cluster from a prohibited namespace", func() {
		JustBeforeEach(func() {
			bindingName = "test-binding-prohibited"
			binding = rabbitmqv1beta1.Binding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: "prohibited",
				},
				Spec: rabbitmqv1beta1.BindingSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
		})
		It("should throw an error about a cluster being prohibited", func() {
			Expect(client.Create(ctx, &binding)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace},
					&binding,
				)

				return binding.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason":  Equal("FailedCreateOrUpdate"),
				"Status":  Equal(corev1.ConditionFalse),
				"Message": ContainSubstring("not allowed to reference"),
			})))
		})
	})

	When("a binding references a cluster from an allowed namespace", func() {
		JustBeforeEach(func() {
			bindingName = "test-binding-allowed"
			binding = rabbitmqv1beta1.Binding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: "allowed",
				},
				Spec: rabbitmqv1beta1.BindingSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
			fakeRabbitMQClient.DeclareBindingReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &binding)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace},
					&binding,
				)

				return binding.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})

	When("a binding references a cluster that allows all namespaces", func() {
		JustBeforeEach(func() {
			bindingName = "test-binding-allowed-when-allow-all"
			binding = rabbitmqv1beta1.Binding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: "prohibited",
				},
				Spec: rabbitmqv1beta1.BindingSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "allow-all-rabbit",
						Namespace: "default",
					},
				},
			}
			fakeRabbitMQClient.DeclareBindingReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &binding)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace},
					&binding,
				)

				return binding.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})
})
