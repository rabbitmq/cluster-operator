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

var _ = Describe("permission-controller", func() {
	var permission rabbitmqv1beta1.Permission
	var user rabbitmqv1beta1.User
	var permissionName string
	var userName string

	When("validating RabbitMQ Client failures with username", func() {
		JustBeforeEach(func() {
			permission = rabbitmqv1beta1.Permission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      permissionName,
					Namespace: "default",
				},
				Spec: rabbitmqv1beta1.PermissionSpec{
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
					permissionName = "test-with-username-http-error"
					fakeRabbitMQClient.UpdatePermissionsInReturns(&http.Response{
						Status:     "418 I'm a teapot",
						StatusCode: 418,
					}, errors.New("a failure"))
				})

				It("sets the status condition", func() {
					Expect(client.Create(ctx, &permission)).To(Succeed())
					EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
							&permission,
						)

						return permission.Status.Conditions
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
					permissionName = "test-with-username-go-error"
					fakeRabbitMQClient.UpdatePermissionsInReturns(nil, errors.New("a go failure"))
				})

				It("sets the status condition to indicate a failure to reconcile", func() {
					Expect(client.Create(ctx, &permission)).To(Succeed())
					EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
							&permission,
						)

						return permission.Status.Conditions
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
					permissionName = "test-with-username-create-success"
					fakeRabbitMQClient.UpdatePermissionsInReturns(&http.Response{
						Status:     "201 Created",
						StatusCode: http.StatusCreated,
					}, nil)
				})

				It("works", func() {
					Expect(client.Create(ctx, &permission)).To(Succeed())
					By("setting the correct finalizer")
					Eventually(komega.Object(&permission)).WithTimeout(2 * time.Second).Should(HaveField("ObjectMeta.Finalizers", ConsistOf("deletion.finalizers.permissions.rabbitmq.com")))

					By("sets the status condition 'Ready' to 'true' ")
					EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
							&permission,
						)

						return permission.Status.Conditions
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
				fakeRabbitMQClient.UpdatePermissionsInReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
				Expect(client.Create(ctx, &permission)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
						&permission,
					)

					return permission.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})

			When("the RabbitMQ Client returns a HTTP error response", func() {
				BeforeEach(func() {
					permissionName = "delete-with-username-permission-http-error"
					fakeRabbitMQClient.ClearPermissionsInReturns(&http.Response{
						Status:     "502 Bad Gateway",
						StatusCode: http.StatusBadGateway,
						Body:       ioutil.NopCloser(bytes.NewBufferString("Hello World")),
					}, nil)
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &permission)).To(Succeed())
					Consistently(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &rabbitmqv1beta1.Permission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeFalse())
					Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete permission"))
				})
			})

			When("the RabbitMQ Client returns a Go error response", func() {
				BeforeEach(func() {
					permissionName = "delete-with-username-go-error"
					fakeRabbitMQClient.ClearPermissionsInReturns(nil, errors.New("some error"))
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &permission)).To(Succeed())
					Consistently(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &rabbitmqv1beta1.Permission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeFalse())
					Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete permission"))
				})
			})

			When("the RabbitMQ Client successfully deletes a permission", func() {
				BeforeEach(func() {
					permissionName = "delete-with-username-permission-success"
					fakeRabbitMQClient.ClearPermissionsInReturns(&http.Response{
						Status:     "204 No Content",
						StatusCode: http.StatusNoContent,
					}, nil)
				})

				It("publishes a normal event", func() {
					Expect(client.Delete(ctx, &permission)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &rabbitmqv1beta1.Permission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeTrue())
					observedEvents := observedEvents()
					Expect(observedEvents).NotTo(ContainElement("Warning FailedDelete failed to delete permission"))
					Expect(observedEvents).To(ContainElement("Normal SuccessfulDelete successfully deleted permission"))
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
			permission = rabbitmqv1beta1.Permission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      permissionName,
					Namespace: "default",
				},
				Spec: rabbitmqv1beta1.PermissionSpec{
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
			fakeRabbitMQClient.UpdatePermissionsInReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			fakeRabbitMQClient.ClearPermissionsInReturns(&http.Response{
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
					permissionName = "test-with-userref-create-not-exist"
					userName = "example-create-not-exist"
				})

				It("sets the status condition 'Ready' to 'true' ", func() {
					Expect(client.Create(ctx, &permission)).To(Succeed())
					EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
							&permission,
						)

						return permission.Status.Conditions
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
					permissionName = "test-with-userref-create-success"
					userName = "example-create-success"
				})

				It("sets the status condition 'Ready' to 'true' ", func() {
					Expect(client.Create(ctx, &user)).To(Succeed())
					Expect(client.Create(ctx, &permission)).To(Succeed())
					EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
						_ = client.Get(
							ctx,
							types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
							&permission,
						)

						return permission.Status.Conditions
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
				Expect(client.Create(ctx, &permission)).To(Succeed())
				EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
					_ = client.Get(
						ctx,
						types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
						&permission,
					)

					return permission.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})

			When("Secret User is removed first", func() {
				BeforeEach(func() {
					permissionName = "test-with-userref-delete-secret"
					userName = "example-delete-secret-first"
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      user.Name + "-user-credentials",
							Namespace: user.Namespace,
						},
					})).To(Succeed())
					Expect(client.Delete(ctx, &permission)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &rabbitmqv1beta1.Permission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeTrue())
					observedEvents := observedEvents()
					Expect(observedEvents).NotTo(ContainElement("Warning FailedDelete failed to delete permission"))
					Expect(observedEvents).To(ContainElement("Normal SuccessfulDelete successfully deleted permission"))
				})
			})

			When("User is removed first", func() {
				BeforeEach(func() {
					permissionName = "test-with-userref-delete-user"
					userName = "example-delete-user-first"
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &user)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: user.Name, Namespace: user.Namespace}, &rabbitmqv1beta1.User{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeTrue())
					Expect(client.Delete(ctx, &permission)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &rabbitmqv1beta1.Permission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeTrue())
					observedEvents := observedEvents()
					Expect(observedEvents).NotTo(ContainElement("Warning FailedDelete failed to delete permission"))
					Expect(observedEvents).To(ContainElement("Normal SuccessfulDelete successfully deleted permission"))
				})
			})

			When("success", func() {
				BeforeEach(func() {
					permissionName = "test-with-userref-delete-success"
					userName = "example-delete-success"
				})

				It("publishes a 'warning' event", func() {
					Expect(client.Delete(ctx, &permission)).To(Succeed())
					Eventually(func() bool {
						err := client.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &rabbitmqv1beta1.Permission{})
						return apierrors.IsNotFound(err)
					}, statusEventsUpdateTimeout).Should(BeTrue())
					observedEvents := observedEvents()
					Expect(observedEvents).NotTo(ContainElement("Warning FailedDelete failed to delete permission"))
					Expect(observedEvents).To(ContainElement("Normal SuccessfulDelete successfully deleted permission"))
				})
			})
		})

		Context("ownerref", func() {
			BeforeEach(func() {
				permissionName = "ownerref-with-userref-test"
				userName = "example-ownerref"
			})

			It("sets the correct deletion ownerref to the object", func() {
				Expect(client.Create(ctx, &user)).To(Succeed())
				Expect(client.Create(ctx, &permission)).To(Succeed())
				Eventually(func() []metav1.OwnerReference {
					var fetched rabbitmqv1beta1.Permission
					err := client.Get(ctx, types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace}, &fetched)
					if err != nil {
						return []metav1.OwnerReference{}
					}
					return fetched.ObjectMeta.OwnerReferences
				}, 5).Should(Not(BeEmpty()))
			})
		})
	})

	When("a permission references a cluster from a prohibited namespace", func() {
		JustBeforeEach(func() {
			permissionName = "test-permission-prohibited"
			permission = rabbitmqv1beta1.Permission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      permissionName,
					Namespace: "prohibited",
				},
				Spec: rabbitmqv1beta1.PermissionSpec{
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
			Expect(client.Create(ctx, &permission)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
					&permission,
				)

				return permission.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason":  Equal("FailedCreateOrUpdate"),
				"Status":  Equal(corev1.ConditionFalse),
				"Message": ContainSubstring("not allowed to reference"),
			})))
		})
	})

	When("a permission references a cluster from an allowed namespace", func() {
		JustBeforeEach(func() {
			permissionName = "test-permission-allowed"
			permission = rabbitmqv1beta1.Permission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      permissionName,
					Namespace: "allowed",
				},
				Spec: rabbitmqv1beta1.PermissionSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "example-rabbit",
						Namespace: "default",
					},
					User:  "example",
					Vhost: "example",
				},
			}
			fakeRabbitMQClient.UpdatePermissionsInReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &permission)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
					&permission,
				)

				return permission.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})

	When("a permission references a cluster that allows all namespaces", func() {
		JustBeforeEach(func() {
			permissionName = "test-permission-allowed-when-allow-all"
			permission = rabbitmqv1beta1.Permission{
				ObjectMeta: metav1.ObjectMeta{
					Name:      permissionName,
					Namespace: "prohibited",
				},
				Spec: rabbitmqv1beta1.PermissionSpec{
					RabbitmqClusterReference: rabbitmqv1beta1.RabbitmqClusterReference{
						Name:      "allow-all-rabbit",
						Namespace: "default",
					},
					User:  "example",
					Vhost: "example",
				},
			}
			fakeRabbitMQClient.UpdatePermissionsInReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("should be created", func() {
			Expect(client.Create(ctx, &permission)).To(Succeed())
			EventuallyWithOffset(1, func() []rabbitmqv1beta1.Condition {
				_ = client.Get(
					ctx,
					types.NamespacedName{Name: permission.Name, Namespace: permission.Namespace},
					&permission,
				)

				return permission.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(rabbitmqv1beta1.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})
})
