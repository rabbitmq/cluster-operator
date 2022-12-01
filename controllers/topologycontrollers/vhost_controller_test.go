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

var _ = Describe("vhost-controller", func() {
	var vhost rabbitmqv1beta1.Vhost
	var vhostName string

	JustBeforeEach(func() {
		vhost = rabbitmqv1beta1.Vhost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vhostName,
				Namespace: "default",
			},
			Spec: rabbitmqv1beta1.VhostSpec{
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: "example-rabbit",
				},
			},
		}
	})

	Context("creation", func() {
		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				vhostName = "test-http-error"
				fakeRabbitMQClient.PutVhostReturns(&http.Response{
					Status:     "418 I'm a teapot",
					StatusCode: 418,
				}, errors.New("a failure"))
			})

			It("sets the status condition", func() {
				Expect(client.Create(ctx, &vhost)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace},
						&vhost,
					)

					return vhost.Status.Conditions
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
				vhostName = "test-go-error"
				fakeRabbitMQClient.PutVhostReturns(nil, errors.New("a go failure"))
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(client.Create(ctx, &vhost)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace},
						&vhost,
					)

					return vhost.Status.Conditions
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
				vhostName = "test-create-success"
				fakeRabbitMQClient.PutVhostReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
			})

			It("works", func() {
				Expect(client.Create(ctx, &vhost)).To(Succeed())
				By("setting the correct finalizer")
				Eventually(komega.Object(&vhost)).WithTimeout(2 * time.Second).Should(HaveField("ObjectMeta.Finalizers", ConsistOf("deletion.finalizers.vhosts.rabbitmq.com")))

				By("sets the status condition 'Ready' to 'true'")
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace},
						&vhost,
					)

					return vhost.Status.Conditions
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
			fakeRabbitMQClient.PutVhostReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			Expect(client.Create(ctx, &vhost)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace},
					&vhost,
				)

				return vhost.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				vhostName = "delete-vhost-http-error"
				fakeRabbitMQClient.DeleteVhostReturns(&http.Response{
					Status:     "502 Bad Gateway",
					StatusCode: http.StatusBadGateway,
					Body:       ioutil.NopCloser(bytes.NewBufferString("Hello World")),
				}, nil)
			})

			It("publishes a 'warning' event", func() {
				Expect(client.Delete(ctx, &vhost)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace}, &rabbitmqv1beta1.Vhost{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete vhost"))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				vhostName = "delete-go-error"
				fakeRabbitMQClient.DeleteVhostReturns(nil, errors.New("some error"))
			})

			It("publishes a 'warning' event", func() {
				Expect(client.Delete(ctx, &vhost)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace}, &rabbitmqv1beta1.Vhost{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete vhost"))
			})
		})

		When("the RabbitMQ Client successfully deletes a vhost", func() {
			BeforeEach(func() {
				vhostName = "delete-vhost-success"
				fakeRabbitMQClient.DeleteVhostReturns(&http.Response{
					Status:     "204 No Content",
					StatusCode: http.StatusNoContent,
				}, nil)
			})

			It("publishes a normal event", func() {
				Expect(client.Delete(ctx, &vhost)).To(Succeed())
				Eventually(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace}, &rabbitmqv1beta1.Vhost{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeTrue())
				Expect(observedEvents()).To(SatisfyAll(
					Not(ContainElement("Warning FailedDelete failed to delete vhost")),
					ContainElement("Normal SuccessfulDelete successfully deleted vhost"),
				))
			})
		})
	})

	When("a vhost references a cluster from a prohibited namespace", func() {
		JustBeforeEach(func() {
			vhostName = "test-vhost-prohibited"
			vhost = rabbitmqv1beta1.Vhost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      vhostName,
					Namespace: "prohibited",
				},
				Spec: rabbitmqv1beta1.VhostSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
		})
		It("should throw an error about a cluster being prohibited", func() {
			Expect(client.Create(ctx, &vhost)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace},
					&vhost,
				)

				return vhost.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason":  Equal("FailedCreateOrUpdate"),
				"Status":  Equal(corev1.ConditionFalse),
				"Message": ContainSubstring("not allowed to reference"),
			})))
		})
	})

	When("a vhost references a cluster from an allowed namespace", func() {
		JustBeforeEach(func() {
			vhostName = "test-vhost-allowed"
			vhost = rabbitmqv1beta1.Vhost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      vhostName,
					Namespace: "allowed",
				},
				Spec: rabbitmqv1beta1.VhostSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
			fakeRabbitMQClient.PutVhostReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &vhost)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace},
					&vhost,
				)

				return vhost.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})

	When("a vhost references a cluster that allows all namespaces", func() {
		JustBeforeEach(func() {
			vhostName = "test-vhost-allowed-when-allow-all"
			vhost = rabbitmqv1beta1.Vhost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      vhostName,
					Namespace: "prohibited",
				},
				Spec: rabbitmqv1beta1.VhostSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "allow-all-rabbit",
						Namespace: "default",
					},
				},
			}
			fakeRabbitMQClient.PutVhostReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &vhost)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: vhost.Name, Namespace: vhost.Namespace},
					&vhost,
				)

				return vhost.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})
})
