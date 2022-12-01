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

var _ = Describe("queue-controller", func() {
	var queue rabbitmqv1beta1.Queue
	var queueName string

	JustBeforeEach(func() {
		queue = rabbitmqv1beta1.Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      queueName,
				Namespace: "default",
			},
			Spec: rabbitmqv1beta1.QueueSpec{
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: "example-rabbit",
				},
			},
		}
	})

	Context("creation", func() {
		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				queueName = "test-http-error"
				fakeRabbitMQClient.DeclareQueueReturns(&http.Response{
					Status:     "418 I'm a teapot",
					StatusCode: 418,
				}, errors.New("a failure"))
			})

			It("sets the status condition", func() {
				Expect(client.Create(ctx, &queue)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace},
						&queue,
					)

					return queue.Status.Conditions
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
				queueName = "test-go-error"
				fakeRabbitMQClient.DeclareQueueReturns(nil, errors.New("a go failure"))
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(client.Create(ctx, &queue)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace},
						&queue,
					)

					return queue.Status.Conditions
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
				queueName = "test-create-success"
				fakeRabbitMQClient.DeclareQueueReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
			})

			It("works", func() {
				Expect(client.Create(ctx, &queue)).To(Succeed())
				By("setting the correct finalizer")
				Eventually(komega.Object(&queue)).WithTimeout(2 * time.Second).Should(HaveField("ObjectMeta.Finalizers", ConsistOf("deletion.finalizers.queues.rabbitmq.com")))

				By("sets the status condition 'Ready' to 'true'")
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace},
						&queue,
					)

					return queue.Status.Conditions
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
			fakeRabbitMQClient.DeclareQueueReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			Expect(client.Create(ctx, &queue)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace},
					&queue,
				)

				return queue.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				queueName = "delete-queue-http-error"
				fakeRabbitMQClient.DeleteQueueReturns(&http.Response{
					Status:     "502 Bad Gateway",
					StatusCode: http.StatusBadGateway,
					Body:       ioutil.NopCloser(bytes.NewBufferString("Hello World")),
				}, nil)
			})

			It("publishes a 'warning' event", func() {
				Expect(client.Delete(ctx, &queue)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace}, &rabbitmqv1beta1.Queue{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete queue"))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				queueName = "delete-go-error"
				fakeRabbitMQClient.DeleteQueueReturns(nil, errors.New("some error"))
			})

			It("publishes a 'warning' event", func() {
				Expect(client.Delete(ctx, &queue)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace}, &rabbitmqv1beta1.Queue{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete queue"))
			})
		})

		When("the RabbitMQ Client successfully deletes a queue", func() {
			BeforeEach(func() {
				queueName = "delete-queue-success"
				fakeRabbitMQClient.DeleteQueueReturns(&http.Response{
					Status:     "204 No Content",
					StatusCode: http.StatusNoContent,
				}, nil)
			})

			It("publishes a normal event", func() {
				Expect(client.Delete(ctx, &queue)).To(Succeed())
				Eventually(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace}, &rabbitmqv1beta1.Queue{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeTrue())
				Expect(observedEvents()).To(SatisfyAll(
					Not(ContainElement("Warning FailedDelete failed to delete queue")),
					ContainElement("Normal SuccessfulDelete successfully deleted queue"),
				))
			})
		})
	})

	When("a queue references a cluster from a prohibited namespace", func() {
		JustBeforeEach(func() {
			queueName = "test-queue-prohibited"
			queue = rabbitmqv1beta1.Queue{
				ObjectMeta: metav1.ObjectMeta{
					Name:      queueName,
					Namespace: "prohibited",
				},
				Spec: rabbitmqv1beta1.QueueSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
		})
		It("should throw an error about a cluster being prohibited", func() {
			Expect(client.Create(ctx, &queue)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace},
					&queue,
				)

				return queue.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason":  Equal("FailedCreateOrUpdate"),
				"Status":  Equal(corev1.ConditionFalse),
				"Message": ContainSubstring("not allowed to reference"),
			})))
		})
	})

	When("a queue references a cluster from an allowed namespace", func() {
		JustBeforeEach(func() {
			queueName = "test-queue-allowed"
			queue = rabbitmqv1beta1.Queue{
				ObjectMeta: metav1.ObjectMeta{
					Name:      queueName,
					Namespace: "allowed",
				},
				Spec: rabbitmqv1beta1.QueueSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
			fakeRabbitMQClient.DeclareQueueReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &queue)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace},
					&queue,
				)

				return queue.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})

	When("a queue references a cluster that allows all namespaces", func() {
		JustBeforeEach(func() {
			queueName = "test-queue-allowed-when-allow-all"
			queue = rabbitmqv1beta1.Queue{
				ObjectMeta: metav1.ObjectMeta{
					Name:      queueName,
					Namespace: "prohibited",
				},
				Spec: rabbitmqv1beta1.QueueSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "allow-all-rabbit",
						Namespace: "default",
					},
				},
			}
			fakeRabbitMQClient.DeclareQueueReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &queue)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace},
					&queue,
				)

				return queue.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})
})
