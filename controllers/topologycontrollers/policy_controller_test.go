package topologycontrollers_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("policy-controller", func() {
	var policy rabbitmqv1beta1.Policy
	var policyName string

	JustBeforeEach(func() {
		policy = rabbitmqv1beta1.Policy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      policyName,
				Namespace: "default",
			},
			Spec: rabbitmqv1beta1.PolicySpec{
				Definition: &runtime.RawExtension{
					Raw: []byte(`{"key":"value"}`),
				},
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: "example-rabbit",
				},
			},
		}
	})

	Context("creation", func() {
		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				policyName = "test-http-error"
				fakeRabbitMQClient.PutPolicyReturns(&http.Response{
					Status:     "418 I'm a teapot",
					StatusCode: 418,
				}, errors.New("a failure"))
			})

			It("sets the status condition", func() {
				Expect(client.Create(ctx, &policy)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace},
						&policy,
					)

					return policy.Status.Conditions
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
				policyName = "test-go-error"
				fakeRabbitMQClient.PutPolicyReturns(nil, errors.New("a go failure"))
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(client.Create(ctx, &policy)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace},
						&policy,
					)

					return policy.Status.Conditions
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
				policyName = "test-create-success"
				fakeRabbitMQClient.PutPolicyReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
			})

			It("works", func() {
				Expect(client.Create(ctx, &policy)).To(Succeed())
				By("setting the correct finalizer")
				Eventually(komega.Object(&policy)).WithTimeout(2 * time.Second).Should(HaveField("ObjectMeta.Finalizers", ConsistOf("deletion.finalizers.policies.rabbitmq.com")))

				By("sets the status condition 'Ready' to 'true' ")
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace},
						&policy,
					)

					return policy.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})
		})
	})

	Context("deletion", func() {
		JustBeforeEach(func() {
			fakeRabbitMQClient.PutPolicyReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			Expect(client.Create(ctx, &policy)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace},
					&policy,
				)

				return policy.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				policyName = "delete-policy-http-error"
				fakeRabbitMQClient.DeletePolicyReturns(&http.Response{
					Status:     "502 Bad Gateway",
					StatusCode: http.StatusBadGateway,
					Body:       ioutil.NopCloser(bytes.NewBufferString("Hello World")),
				}, nil)
			})

			It("publishes a 'warning' event", func() {
				Expect(client.Delete(ctx, &policy)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace}, &rabbitmqv1beta1.Policy{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete policy"))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				policyName = "delete-go-error"
				fakeRabbitMQClient.DeletePolicyReturns(nil, errors.New("some error"))
			})

			It("publishes a 'warning' event", func() {
				Expect(client.Delete(ctx, &policy)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace}, &rabbitmqv1beta1.Policy{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete policy"))
			})
		})

		When("the RabbitMQ Client successfully deletes a policy", func() {
			BeforeEach(func() {
				policyName = "delete-policy-success"
				fakeRabbitMQClient.DeletePolicyReturns(&http.Response{
					Status:     "204 No Content",
					StatusCode: http.StatusNoContent,
				}, nil)
			})

			It("publishes a normal event", func() {
				Expect(client.Delete(ctx, &policy)).To(Succeed())
				Eventually(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace}, &rabbitmqv1beta1.Policy{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeTrue())
				Expect(observedEvents()).To(SatisfyAll(
					Not(ContainElement("Warning FailedDelete failed to delete policy")),
					ContainElement("Normal SuccessfulDelete successfully deleted policy"),
				))
			})
		})
	})

	When("a policy references a cluster from a prohibited namespace", func() {
		JustBeforeEach(func() {
			policyName = "test-policy-prohibited"
			policy = rabbitmqv1beta1.Policy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      policyName,
					Namespace: "prohibited",
				},
				Spec: rabbitmqv1beta1.PolicySpec{
					Definition: &runtime.RawExtension{
						Raw: []byte(`{"key":"value"}`),
					},
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
		})
		It("should throw an error about a cluster being prohibited", func() {
			Expect(client.Create(ctx, &policy)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace},
					&policy,
				)

				return policy.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason":  Equal("FailedCreateOrUpdate"),
				"Status":  Equal(corev1.ConditionFalse),
				"Message": ContainSubstring("not allowed to reference"),
			})))
		})
	})

	When("a policy references a cluster from an allowed namespace", func() {
		JustBeforeEach(func() {
			policyName = "test-policy-allowed"
			policy = rabbitmqv1beta1.Policy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      policyName,
					Namespace: "allowed",
				},
				Spec: rabbitmqv1beta1.PolicySpec{
					Definition: &runtime.RawExtension{
						Raw: []byte(`{"key":"value"}`),
					},
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
			fakeRabbitMQClient.PutPolicyReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &policy)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace},
					&policy,
				)

				return policy.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})

	When("a policy references a cluster that allows all namespaces", func() {
		JustBeforeEach(func() {
			policyName = "test-policy-allowed-when-allow-all"
			policy = rabbitmqv1beta1.Policy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      policyName,
					Namespace: "prohibited",
				},
				Spec: rabbitmqv1beta1.PolicySpec{
					Definition: &runtime.RawExtension{
						Raw: []byte(`{"key":"value"}`),
					},
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "allow-all-rabbit",
						Namespace: "default",
					},
				},
			}
			fakeRabbitMQClient.PutPolicyReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &policy)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace},
					&policy,
				)

				return policy.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})
})
