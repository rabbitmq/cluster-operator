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

var _ = Describe("topicpermission-controller", func() {
	var topicperm rabbitmqv1beta1.TopicPermission
	var user rabbitmqv1beta1.User
	var name string
	var userName string

	When("validating RabbitMQ Client failures with username", func() {
		JustBeforeEach(func() {
			topicperm = rabbitmqv1beta1.TopicPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "default",
				},
				Spec: rabbitmqv1beta1.TopicPermissionSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name: "example-rabbit",
					},
					User:  "example",
					Vhost: "example",
				},
			}
		})

		Context("creation", func() {
			When("the RabbitMQ Client returns a HTTP error response", func() {
				BeforeEach(func() {
					name = "test-with-username-http-error"
					fakeRabbitMQClient.UpdateTopicPermissionsInReturns(&http.Response{
						Status:     "418 I'm a teapot",
						StatusCode: 418,
					}, errors.New("a failure"))
				})

				It("sets the status condition", func() {
					Expect(client.Create(ctx, &topicperm)).To(Succeed())
					EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
							&topicperm,
						)

						return topicperm.Status.Conditions
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
					name = "test-with-username-go-error"
					fakeRabbitMQClient.UpdateTopicPermissionsInReturns(nil, errors.New("a go failure"))
				})

				It("sets the status condition to indicate a failure to reconcile", func() {
					Expect(client.Create(ctx, &topicperm)).To(Succeed())
					EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
							&topicperm,
						)

						return topicperm.Status.Conditions
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
					name = "test-with-username-create-success"
					fakeRabbitMQClient.UpdateTopicPermissionsInReturns(&http.Response{
						Status:     "201 Created",
						StatusCode: http.StatusCreated,
					}, nil)
				})

				It("works", func() {
					Expect(client.Create(ctx, &topicperm)).To(Succeed())
					By("setting the correct finalizer")
					Eventually(komega.Object(&topicperm)).WithTimeout(2 * time.Second).Should(HaveField("ObjectMeta.Finalizers", ConsistOf("deletion.finalizers.topicpermissions.rabbitmq.com")))

					By("sets the status condition 'Ready' to 'true' ")
					EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
							&topicperm,
						)

						return topicperm.Status.Conditions
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
				fakeRabbitMQClient.UpdateTopicPermissionsInReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
				Expect(client.Create(ctx, &topicperm)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
						&topicperm,
					)

					return topicperm.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})

			When("the RabbitMQ Client returns a HTTP error response", func() {
				BeforeEach(func() {
					name = "delete-with-username-topicperm-http-error"
					fakeRabbitMQClient.DeleteTopicPermissionsInReturns(&http.Response{
						Status:     "502 Bad Gateway",
						StatusCode: http.StatusBadGateway,
						Body:       ioutil.NopCloser(bytes.NewBufferString("Hello World")),
					}, nil)
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &topicperm)).To(Succeed())
					Consistently(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace}, &rabbitmqv1beta1.TopicPermission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeFalse())
					Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete topicpermission"))
				})
			})

			When("the RabbitMQ Client returns a Go error response", func() {
				BeforeEach(func() {
					name = "delete-with-username-go-error"
					fakeRabbitMQClient.DeleteTopicPermissionsInReturns(nil, errors.New("some error"))
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &topicperm)).To(Succeed())
					Consistently(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace}, &rabbitmqv1beta1.TopicPermission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeFalse())
					Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete topicpermission"))
				})
			})

			When("the RabbitMQ Client successfully deletes a topicperm", func() {
				BeforeEach(func() {
					name = "delete-with-username-topicperm-success"
					fakeRabbitMQClient.DeleteTopicPermissionsInReturns(&http.Response{
						Status:     "204 No Content",
						StatusCode: http.StatusNoContent,
					}, nil)
				})

				It("publishes a normal event", func() {
					Expect(client.Delete(ctx, &topicperm)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace}, &rabbitmqv1beta1.TopicPermission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeTrue())
					observedEvents := observedEvents()
					Expect(observedEvents).NotTo(ContainElement("Warning FailedDelete failed to delete topicpermission"))
					Expect(observedEvents).To(ContainElement("Normal SuccessfulDelete successfully deleted topicpermission"))
				})
			})
		})
	})

	When("validating RabbitMQ Client failures with userRef", func() {
		JustBeforeEach(func() {
			user = rabbitmqv1beta1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      userName,
					Namespace: "default",
				},
				Spec: rabbitmqv1beta1.UserSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
				},
			}
			topicperm = rabbitmqv1beta1.TopicPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "default",
				},
				Spec: rabbitmqv1beta1.TopicPermissionSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
					UserReference: &corev1.LocalObjectReference{
						Name: userName,
					},
					Vhost: "example",
				},
			}
			fakeRabbitMQClient.UpdateTopicPermissionsInReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			fakeRabbitMQClient.DeleteTopicPermissionsInReturns(&http.Response{
				Status:     "204 No Content",
				StatusCode: http.StatusNoContent,
			}, nil)
			fakeRabbitMQClient.PutUserReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			fakeRabbitMQClient.DeleteUserReturns(&http.Response{
				Status:     "204 No Content",
				StatusCode: http.StatusNoContent,
			}, nil)
		})

		Context("creation", func() {
			When("user not exist", func() {
				BeforeEach(func() {
					name = "test-with-userref-create-not-exist"
					userName = "topic-perm-example-create-not-exist"
				})

				It("sets the status condition 'Ready' to 'true' ", func() {
					Expect(client.Create(ctx, &topicperm)).To(Succeed())
					EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
							&topicperm,
						)

						return topicperm.Status.Conditions
					}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(rabbitmqv1beta1.ConditionType("Ready")),
						"Reason":  Equal("FailedCreateOrUpdate"),
						"Message": Equal("failed create Permission, missing User"),
						"Status":  Equal(corev1.ConditionFalse),
					})))
				})
			})

			When("success", func() {
				BeforeEach(func() {
					name = "test-with-userref-create-success"
					userName = "topic-perm-example-create-success"
				})

				It("sets the status condition 'Ready' to 'true' ", func() {
					Expect(client.Create(ctx, &user)).To(Succeed())
					Expect(client.Create(ctx, &topicperm)).To(Succeed())
					EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
							&topicperm,
						)

						return topicperm.Status.Conditions
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
				Expect(client.Create(ctx, &user)).To(Succeed())
				Expect(client.Create(ctx, &topicperm)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
						&topicperm,
					)

					return topicperm.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})

			When("Secret User is removed first", func() {
				BeforeEach(func() {
					name = "test-with-userref-delete-secret"
					userName = "topic-perm-example-delete-secret-first"
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      user.Name + "-user-credentials",
							Namespace: user.Namespace,
						},
					})).To(Succeed())
					Expect(client.Delete(ctx, &topicperm)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace}, &rabbitmqv1beta1.TopicPermission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeTrue())
					observedEvents := observedEvents()
					Expect(observedEvents).NotTo(ContainElement("Warning FailedDelete failed to delete topicpermission"))
					Expect(observedEvents).To(ContainElement("Normal SuccessfulDelete successfully deleted topicpermission"))
				})
			})

			When("User is removed first", func() {
				BeforeEach(func() {
					name = "test-with-userref-delete-user"
					userName = "topic-perm-example-delete-user-first"
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &user)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &rabbitmqv1beta1.User{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeTrue())
					Expect(client.Delete(ctx, &topicperm)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace}, &rabbitmqv1beta1.TopicPermission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeTrue())
					observedEvents := observedEvents()
					Expect(observedEvents).NotTo(ContainElement("Warning FailedDelete failed to delete topicpermission"))
					Expect(observedEvents).To(ContainElement("Normal SuccessfulDelete successfully deleted topicpermission"))
				})
			})

			When("success", func() {
				BeforeEach(func() {
					name = "test-with-userref-delete-success"
					userName = "topic-perm-example-delete-success"
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &topicperm)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace}, &rabbitmqv1beta1.TopicPermission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeTrue())
					observedEvents := observedEvents()
					Expect(observedEvents).NotTo(ContainElement("Warning FailedDelete failed to delete topicpermission"))
					Expect(observedEvents).To(ContainElement("Normal SuccessfulDelete successfully deleted topicpermission"))
				})
			})
		})

		Context("ownerref", func() {
			BeforeEach(func() {
				name = "ownerref-with-userref-test"
				userName = "topic-perm-topic-perm-user"
			})

			It("sets the correct deletion ownerref to the object", func() {
				Expect(client.Create(ctx, &user)).To(Succeed())
				Expect(client.Create(ctx, &topicperm)).To(Succeed())
				Eventually(func() []metav1.OwnerReference {
					var fetched rabbitmqv1beta1.TopicPermission
					err := client.Get(ctx, types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace}, &fetched)
					if err != nil {
						return []metav1.OwnerReference{}
					}
					return fetched.ObjectMeta.OwnerReferences
				}, 5).Should(Not(BeEmpty()))
			})
		})
	})

	When("a topic permission references a cluster from a prohibited namespace", func() {
		JustBeforeEach(func() {
			name = "test-topicperm-prohibited"
			topicperm = rabbitmqv1beta1.TopicPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "prohibited",
				},
				Spec: rabbitmqv1beta1.TopicPermissionSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
					User:  "example",
					Vhost: "example",
				},
			}
		})
		It("should throw an error about a cluster being prohibited", func() {
			Expect(client.Create(ctx, &topicperm)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
					&topicperm,
				)

				return topicperm.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason":  Equal("FailedCreateOrUpdate"),
				"Status":  Equal(corev1.ConditionFalse),
				"Message": ContainSubstring("not allowed to reference"),
			})))
		})
	})

	When("a topic permission references a cluster from an allowed namespace", func() {
		JustBeforeEach(func() {
			name = "test-topicperm-allowed"
			topicperm = rabbitmqv1beta1.TopicPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "allowed",
				},
				Spec: rabbitmqv1beta1.TopicPermissionSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
					User:  "example",
					Vhost: "example",
				},
			}
			fakeRabbitMQClient.UpdateTopicPermissionsInReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &topicperm)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
					&topicperm,
				)

				return topicperm.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})

	When("a topic permission references a cluster that allows all namespaces", func() {
		JustBeforeEach(func() {
			name = "test-topicperm-allowed-when-allow-all"
			topicperm = rabbitmqv1beta1.TopicPermission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "prohibited",
				},
				Spec: rabbitmqv1beta1.TopicPermissionSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "allow-all-rabbit",
						Namespace: "default",
					},
					User:  "example",
					Vhost: "example",
				},
			}
			fakeRabbitMQClient.UpdateTopicPermissionsInReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &topicperm)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: topicperm.Name, Namespace: topicperm.Namespace},
					&topicperm,
				)

				return topicperm.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})
})
