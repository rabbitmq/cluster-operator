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

var _ = Describe("UserController", func() {
	var user rabbitmqv1beta1.User
	var userName string

	JustBeforeEach(func() {
		user = rabbitmqv1beta1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      userName,
				Namespace: "default",
			},
			Spec: rabbitmqv1beta1.UserSpec{
				RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
					Name: "example-rabbit",
				},
			},
		}
	})
	When("creating a user", func() {
		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				userName = "test-user-http-error"
				fakeRabbitMQClient.PutUserReturns(&http.Response{
					Status:     "418 I'm a teapot",
					StatusCode: 418,
				}, errors.New("some HTTP error"))
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(client.Create(ctx, &user)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: user.Name, Namespace: user.Namespace},
						&user,
					)

					return user.Status.Conditions
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
				userName = "test-user-go-error"
				fakeRabbitMQClient.PutUserReturns(nil, errors.New("hit a exception"))
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(client.Create(ctx, &user)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: user.Name, Namespace: user.Namespace},
						&user,
					)

					return user.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(rabbitmqv1beta1.ConditionType("Ready")),
					"Reason":  Equal("FailedCreateOrUpdate"),
					"Status":  Equal(corev1.ConditionFalse),
					"Message": ContainSubstring("hit a exception"),
				})))
			})
		})

		When("the RabbitMQ Client successfully creates a user", func() {
			BeforeEach(func() {
				userName = "test-user-success"
				fakeRabbitMQClient.PutUserReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
			})

			It("works", func() {
				Expect(client.Create(ctx, &user)).To(Succeed())
				By("setting the correct finalizer")
				Eventually(komega.Object(&user)).WithTimeout(2 * time.Second).Should(HaveField("ObjectMeta.Finalizers", ConsistOf("deletion.finalizers.users.rabbitmq.com")))

				By("sets the status condition 'Ready' to 'true'")
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: user.Name, Namespace: user.Namespace},
						&user,
					)

					return user.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
				Expect(user.Status.Username).Should(Not(BeEmpty()))
			})

			It("sets an owner reference and does not block owner deletion", func() {
				user.Name = "test-owner-reference"
				Expect(client.Create(ctx, &user)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: user.Name, Namespace: user.Namespace},
						&user,
					)

					return user.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})), "User should have been created and have a True Ready condition")

				generatedSecret := &corev1.Secret{}
				EventuallyWithOffset(1, func() error {
					return client.Get(ctx,
						types.NamespacedName{
							Namespace: user.Namespace,
							Name:      user.Name + "-user-credentials",
						}, generatedSecret)
				}, 10*time.Second).ShouldNot(HaveOccurred())

				for _, ref := range generatedSecret.ObjectMeta.OwnerReferences {
					Expect(ref).To(MatchFields(IgnoreExtras, Fields{
						"BlockOwnerDeletion": PointTo(BeFalse()),
					}))
				}
			})
		})
	})

	When("deleting a user", func() {
		JustBeforeEach(func() {
			fakeRabbitMQClient.PutUserReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			Expect(client.Create(ctx, &user)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: user.Name, Namespace: user.Namespace},
					&user,
				)

				return user.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				userName = "delete-user-http-error"
				fakeRabbitMQClient.DeleteUserReturns(&http.Response{
					Status:     "502 Bad Gateway",
					StatusCode: http.StatusBadGateway,
					Body:       ioutil.NopCloser(bytes.NewBufferString("Hello World")),
				}, nil)
			})

			It("raises an event to indicate a failure to delete", func() {
				Expect(client.Delete(ctx, &user)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &rabbitmqv1beta1.User{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete user"))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				userName = "delete-user-go-error"
				fakeRabbitMQClient.DeleteUserReturns(nil, errors.New("some error"))
			})

			It("raises an event to indicate a failure to delete", func() {
				Expect(client.Delete(ctx, &user)).To(Succeed())
				Consistently(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &rabbitmqv1beta1.User{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete user"))
			})
		})

		When("the RabbitMQ Client successfully deletes a user without secret", func() {
			BeforeEach(func() {
				userName = "delete-user-success-without-secret-user-credentials"
				fakeRabbitMQClient.DeleteUserReturns(&http.Response{
					Status:     "204 No Content",
					StatusCode: http.StatusNoContent,
				}, nil)
			})

			It("raises an event to indicate a successful deletion", func() {
				Expect(client.Delete(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      user.Name + "-user-credentials",
						Namespace: user.Namespace,
					},
				})).To(Succeed())
				Expect(client.Delete(ctx, &user)).To(Succeed())
				Eventually(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &rabbitmqv1beta1.User{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeTrue())

				Expect(observedEvents()).To(SatisfyAll(
					Not(ContainElement("Warning FailedDelete failed to delete user")),
					ContainElement("Normal SuccessfulDelete successfully deleted user"),
				))
			})
		})

		When("the RabbitMQ Client successfully deletes a user", func() {
			BeforeEach(func() {
				userName = "delete-user-success"
				fakeRabbitMQClient.DeleteUserReturns(&http.Response{
					Status:     "204 No Content",
					StatusCode: http.StatusNoContent,
				}, nil)
			})

			It("raises an event to indicate a successful deletion", func() {
				Expect(client.Delete(ctx, &user)).To(Succeed())
				Eventually(func() bool {
					err := client.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &rabbitmqv1beta1.User{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeTrue())

				Expect(observedEvents()).To(SatisfyAll(
					Not(ContainElement("Warning FailedDelete failed to delete User")),
					ContainElement("Normal SuccessfulDelete successfully deleted user"),
				))
			})
		})
	})

	When("a user references a cluster from a prohibited namespace", func() {
		JustBeforeEach(func() {
			userName = "test-user-prohibited"
			user = rabbitmqv1beta1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      userName,
					Namespace: "prohibited",
				},
				Spec: rabbitmqv1beta1.UserSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
		})
		It("should throw an error about a cluster being prohibited", func() {
			Expect(client.Create(ctx, &user)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: user.Name, Namespace: user.Namespace},
					&user,
				)

				return user.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason":  Equal("FailedCreateOrUpdate"),
				"Status":  Equal(corev1.ConditionFalse),
				"Message": ContainSubstring("not allowed to reference"),
			})))
		})
	})

	When("a user references a cluster from an allowed namespace", func() {
		JustBeforeEach(func() {
			userName = "test-user-allowed"
			user = rabbitmqv1beta1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      userName,
					Namespace: "allowed",
				},
				Spec: rabbitmqv1beta1.UserSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
			fakeRabbitMQClient.PutUserReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &user)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: user.Name, Namespace: user.Namespace},
					&user,
				)

				return user.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})

	When("a user references a cluster that allows all namespaces", func() {
		JustBeforeEach(func() {
			userName = "test-user-allowed-when-allow-all"
			user = rabbitmqv1beta1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      userName,
					Namespace: "prohibited",
				},
				Spec: rabbitmqv1beta1.UserSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "allow-all-rabbit",
						Namespace: "default",
					},
				},
			}
			fakeRabbitMQClient.PutUserReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &user)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: user.Name, Namespace: user.Namespace},
					&user,
				)

				return user.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})
})
