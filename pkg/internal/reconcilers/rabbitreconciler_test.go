package reconcilers_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/pkg/apis/rabbitmq/v1beta1"
	. "github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/reconcilers"
	"github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/reconcilers/reconcilersfakes"
	resourcegenerator "github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/resourcegenerator"
	"github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/resourcemanager/resourcemanagerfakes"
	"github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/secret/secretfakes"
	"k8s.io/api/apps/v1beta1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Rabbitreconciler", func() {
	var (
		reconciler      *RabbitReconciler
		repository      *reconcilersfakes.FakeRepository
		notFoundError   *apierrors.StatusError
		badRequestError *apierrors.StatusError
		resourceManager *resourcemanagerfakes.FakeResourceManager
		rabbitSecret    *secretfakes.FakeSecret
	)

	Context("Reconcile", func() {
		BeforeEach(func() {
			repository = new(reconcilersfakes.FakeRepository)
			resourceManager = new(resourcemanagerfakes.FakeResourceManager)
			rabbitSecret = new(secretfakes.FakeSecret)

			reconciler = NewRabbitReconciler(repository, resourceManager, rabbitSecret)

			groupResource := schema.GroupResource{}
			notFoundError = apierrors.NewNotFound(groupResource, "rabbit")
			badRequestError = apierrors.NewBadRequest("fake bad request")
		})
		It("returns an empty object if the instance is not found", func() {
			repository.GetReturns(notFoundError)

			result, resultErr := reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})

			Expect(result).To(Equal(reconcile.Result{}))
			Expect(resultErr).To(BeNil())
		})
		It("returns an empty object and an error in case of unexpected error when loading the instance", func() {
			repository.GetReturns(badRequestError)

			result, resultErr := reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})

			Expect(result).To(Equal(reconcile.Result{}))
			Expect(resultErr).To(Equal(badRequestError))
		})

		It("returns an error if the secret is not parsable", func() {
			repository.GetCalls(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch o := obj.(type) {
				case *rabbitmqv1beta1.RabbitmqCluster:
					o.Spec.Plan = "ha"
					o.Name = "rabbit"
					o.Namespace = "default"
					return nil
				}
				return nil
			})
			err := errors.New("secret parsing failed")
			newSecret := &v1.Secret{}
			rabbitSecret.NewReturns(newSecret, err)

			result, resultErr := reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})
			instanceUsed := rabbitSecret.NewArgsForCall(0)

			Expect(instanceUsed.Spec.Plan).To(Equal("ha"))
			Expect(instanceUsed.Name).To(Equal("rabbit"))
			Expect(instanceUsed.Namespace).To(Equal("default"))

			Expect(result).To(Equal(reconcile.Result{}))
			Expect(resultErr).To(Equal(err))
		})

		It("returns an error if the controller reference for the secret cannot be set", func() {
			repository.GetCalls(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch o := obj.(type) {
				case *rabbitmqv1beta1.RabbitmqCluster:
					o.Spec.Plan = "ha"
					o.Name = "rabbit"
					o.Namespace = "default"
					return nil
				}
				return nil
			})
			newSecret := &v1.Secret{}
			rabbitSecret.NewReturns(newSecret, nil)
			err := errors.New("referencing failed")
			repository.SetControllerReferenceReturns(err)

			result, resultErr := reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})

			Expect(result).To(Equal(reconcile.Result{}))
			Expect(resultErr).To(Equal(err))
		})

		It("returns an error if retrieving the secret fails", func() {
			repository.GetCalls(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch o := obj.(type) {
				case *rabbitmqv1beta1.RabbitmqCluster:
					o.Spec.Plan = "ha"
					o.Namespace = "rabbit-namespace"
					o.Name = "rabbit"
					return nil
				case *v1.Secret:
					return badRequestError
				default:
					return errors.New("Test error")
				}
			})
			newSecret := &v1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbit",
					Namespace: "rabbit-namespace",
				},
			}
			rabbitSecret.NewReturns(newSecret, nil)
			repository.SetControllerReferenceReturns(nil)

			result, resultErr := reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})

			Expect(result).To(Equal(reconcile.Result{}))
			Expect(resultErr).To(Equal(badRequestError))
		})

		It("creates the secret if it does not exist", func() {
			var instance *rabbitmqv1beta1.RabbitmqCluster
			repository.GetCalls(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch o := obj.(type) {
				case *rabbitmqv1beta1.RabbitmqCluster:
					o.Spec.Plan = "ha"
					o.Namespace = "rabbit-namespace"
					o.Name = "rabbit"
					instance = o
					return nil
				case *v1.Secret:
					return notFoundError
				default:
					return errors.New("Test error")
				}
			})
			newSecret := &v1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbit",
					Namespace: "rabbit-namespace",
				},
			}
			rabbitSecret.NewReturns(newSecret, nil)
			repository.SetControllerReferenceReturns(nil)

			reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})
			ctx, resourceObject := repository.CreateArgsForCall(0)
			requestInstance := rabbitSecret.NewArgsForCall(0)

			Expect(repository.GetCallCount()).To(Equal(2))
			Expect(repository.CreateCallCount()).To(Equal(1))
			Expect(ctx).To(Equal(context.TODO()))
			Expect(resourceObject).To(Equal(newSecret))

			Expect(instance).To(Equal(requestInstance))
		})
		It("returns an error if secret creation fails", func() {
			repository.GetCalls(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch o := obj.(type) {
				case *rabbitmqv1beta1.RabbitmqCluster:
					o.Spec.Plan = "ha"
					o.Namespace = "rabbit-namespace"
					o.Name = "rabbit"
					return nil
				case *v1.Secret:
					return notFoundError
				default:
					return errors.New("Test error")
				}
			})
			newSecret := &v1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbit",
					Namespace: "rabbit-namespace",
				},
			}
			rabbitSecret.NewReturns(newSecret, nil)
			repository.SetControllerReferenceReturns(nil)
			err := errors.New("creation error")
			repository.CreateReturns(err)

			result, resultErr := reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})

			Expect(result).To(Equal(reconcile.Result{}))
			Expect(resultErr).To(Equal(err))
		})

		It("returns an empty object and an error when kustomize fails", func() {
			repository.GetCalls(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch o := obj.(type) {
				case *rabbitmqv1beta1.RabbitmqCluster:
					o.Spec.Plan = "ha"
					return nil
				case *v1.Secret:
					return nil
				}
				return nil
			})

			newSecret := &v1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbit",
					Namespace: "rabbit-namespace",
				},
			}
			rabbitSecret.NewReturns(newSecret, nil)
			err := errors.New("whatever")
			resourceManager.ConfigureReturns(make([]resourcegenerator.TargetResource, 0), err)
			result, resultErr := reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})

			Expect(result).To(Equal(reconcile.Result{}))
			Expect(resultErr).To(Equal(err))
		})
		It("returns an empty object and an error when referencing a resource fails", func() {
			repository.GetCalls(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch o := obj.(type) {
				case *rabbitmqv1beta1.RabbitmqCluster:
					o.Spec.Plan = "ha"
					return nil
				case *v1.Secret:
					return nil
				}
				return nil
			})
			newSecret := &v1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbit",
					Namespace: "rabbit-namespace",
				},
			}
			rabbitSecret.NewReturns(newSecret, nil)
			resource := resourcegenerator.TargetResource{ResourceObject: &v1.Service{}, EmptyResource: &v1.Service{}, Name: "", Namespace: ""}
			resources := []resourcegenerator.TargetResource{resource}
			resourceManager.ConfigureReturns(resources, nil)
			err := errors.New("referencing failed")
			repository.SetControllerReferenceReturns(err)
			result, resultErr := reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})

			Expect(result).To(Equal(reconcile.Result{}))
			Expect(resultErr).To(Equal(err))
		})
		It("creates the resource if it is not found", func() {
			var instance *rabbitmqv1beta1.RabbitmqCluster
			repository.GetCalls(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch o := obj.(type) {
				case *rabbitmqv1beta1.RabbitmqCluster:
					o.Spec.Plan = "ha"
					o.Namespace = "rabbit-namespace"
					o.Name = "rabbit"
					instance = o
					return nil
				case *v1.Service:
					return notFoundError
				case *v1.Secret:
					return nil
				default:
					return errors.New("Test error")
				}
			})
			newSecret := &v1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbit",
					Namespace: "rabbit-namespace",
				},
			}
			rabbitSecret.NewReturns(newSecret, nil)
			resource := resourcegenerator.TargetResource{ResourceObject: &v1.Service{}, EmptyResource: &v1.Service{}, Name: "", Namespace: ""}
			resources := []resourcegenerator.TargetResource{resource}
			resourceManager.ConfigureReturns(resources, nil)
			repository.SetControllerReferenceReturns(nil)

			reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})
			ctx, resourceObject := repository.CreateArgsForCall(0)
			requestInstance := resourceManager.ConfigureArgsForCall(0)

			Expect(repository.CreateCallCount()).To(Equal(1))
			Expect(ctx).To(Equal(context.TODO()))
			Expect(resourceObject).To(Equal(resource.ResourceObject))

			Expect(instance).To(Equal(requestInstance))
		})

		It("creates multiple resources if they are not found", func() {
			repository.GetCalls(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch o := obj.(type) {
				case *rabbitmqv1beta1.RabbitmqCluster:
					o.Spec.Plan = "ha"
					return nil
				case *v1.Service:
					return notFoundError
				case *v1beta1.StatefulSet:
					return notFoundError
				case *v1.Secret:
					return nil
				default:
					return errors.New("Test error")
				}
			})
			newSecret := &v1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbit",
					Namespace: "rabbit-namespace",
				},
			}
			rabbitSecret.NewReturns(newSecret, nil)
			resource1 := resourcegenerator.TargetResource{ResourceObject: &v1.Service{}, EmptyResource: &v1.Service{}, Name: "", Namespace: ""}
			resource2 := resourcegenerator.TargetResource{ResourceObject: &v1beta1.StatefulSet{}, EmptyResource: &v1beta1.StatefulSet{}, Name: "", Namespace: ""}
			resources := []resourcegenerator.TargetResource{resource1, resource2}
			resourceManager.ConfigureReturns(resources, nil)
			repository.SetControllerReferenceReturns(nil)

			reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})
			ctx1, resourceObject1 := repository.CreateArgsForCall(0)
			ctx2, resourceObject2 := repository.CreateArgsForCall(1)

			Expect(repository.CreateCallCount()).To(Equal(2))

			Expect(ctx1).To(Equal(context.TODO()))
			Expect(resourceObject1).To(Equal(resource1.ResourceObject))

			Expect(ctx2).To(Equal(context.TODO()))
			Expect(resourceObject2).To(Equal(resource2.ResourceObject))
		})
		It("returns an empty result if it cannot create the resource", func() {
			repository.GetCalls(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch o := obj.(type) {
				case *rabbitmqv1beta1.RabbitmqCluster:
					o.Spec.Plan = "ha"
					o.Namespace = "rabbit-namespace"
					o.Name = "rabbit"
					return nil
				case *v1.Service:
					return notFoundError
				case *v1.Secret:
					return nil
				default:
					return errors.New("Test error")
				}
			})
			newSecret := &v1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbit",
					Namespace: "rabbit-namespace",
				},
			}
			rabbitSecret.NewReturns(newSecret, nil)
			resource := resourcegenerator.TargetResource{ResourceObject: &v1.Service{}, EmptyResource: &v1.Service{}, Name: "", Namespace: ""}
			resources := []resourcegenerator.TargetResource{resource}
			resourceManager.ConfigureReturns(resources, nil)
			repository.SetControllerReferenceReturns(nil)
			createError := errors.New("fake error")
			repository.CreateReturns(createError)

			result, resultErr := reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})

			Expect(result).To(Equal(reconcile.Result{}))
			Expect(resultErr).To(Equal(createError))
		})
		It("returns an empty object and an error in case of unexpected error when loading the resource", func() {
			repository.GetCalls(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch o := obj.(type) {
				case *rabbitmqv1beta1.RabbitmqCluster:
					o.Spec.Plan = "ha"
					return nil
				case *v1.Service:
					return badRequestError
				case *v1.Secret:
					return nil
				default:
					return errors.New("Test error")
				}
			})
			newSecret := &v1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbit",
					Namespace: "rabbit-namespace",
				},
			}
			rabbitSecret.NewReturns(newSecret, nil)
			resource := resourcegenerator.TargetResource{ResourceObject: &v1.Service{}, EmptyResource: &v1.Service{}, Name: "", Namespace: ""}
			resources := []resourcegenerator.TargetResource{resource}
			resourceManager.ConfigureReturns(resources, nil)
			repository.SetControllerReferenceReturns(nil)

			result, resultErr := reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})

			Expect(result).To(Equal(reconcile.Result{}))
			Expect(resultErr).To(Equal(badRequestError))
		})
		It("checks for changes to existing stateful set and updates the cluster", func() {
			repository.GetCalls(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch o := obj.(type) {
				case *rabbitmqv1beta1.RabbitmqCluster:
					o.Spec.Plan = "ha"
					return nil
				case *v1beta1.StatefulSet:
					return nil
				case *v1.Secret:
					return nil
				default:
					return errors.New("Test error")
				}
			})
			three := int32(3)
			statefulSet := &v1beta1.StatefulSet{
				Spec: v1beta1.StatefulSetSpec{
					Replicas: &three,
				},
			}
			two := int32(2)
			foundStatefulSet := &v1beta1.StatefulSet{
				Spec: v1beta1.StatefulSetSpec{
					Replicas: &two,
				},
			}
			newSecret := &v1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbit",
					Namespace: "rabbit-namespace",
				},
			}
			rabbitSecret.NewReturns(newSecret, nil)
			resource := resourcegenerator.TargetResource{ResourceObject: statefulSet, EmptyResource: foundStatefulSet, Name: "", Namespace: ""}
			resources := []resourcegenerator.TargetResource{resource}
			resourceManager.ConfigureReturns(resources, nil)
			repository.SetControllerReferenceReturns(nil)
			repository.UpdateReturns(nil)

			result, resultErr := reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})

			Expect(repository.UpdateCallCount()).To(Equal(1))
			ctx, object := repository.UpdateArgsForCall(0)

			Expect(ctx).To(Equal(context.TODO()))
			Expect(object).To(Equal(statefulSet))
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(resultErr).To(BeNil())
		})
		It("does not update if the resource has been created", func() {
			repository.GetCalls(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch o := obj.(type) {
				case *rabbitmqv1beta1.RabbitmqCluster:
					o.Spec.Plan = "ha"
					return nil
				case *v1.Secret:
					return nil
				case *v1beta1.StatefulSet:
					return notFoundError
				default:
					return errors.New("Test error")
				}
			})
			three := int32(3)
			statefulSet := &v1beta1.StatefulSet{
				Spec: v1beta1.StatefulSetSpec{
					Replicas: &three,
				},
			}
			two := int32(2)
			foundStatefulSet := &v1beta1.StatefulSet{
				Spec: v1beta1.StatefulSetSpec{
					Replicas: &two,
				},
			}
			newSecret := &v1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbit",
					Namespace: "rabbit-namespace",
				},
			}
			rabbitSecret.NewReturns(newSecret, nil)
			resource := resourcegenerator.TargetResource{ResourceObject: statefulSet, EmptyResource: foundStatefulSet, Name: "", Namespace: ""}
			resources := []resourcegenerator.TargetResource{resource}
			resourceManager.ConfigureReturns(resources, nil)
			repository.SetControllerReferenceReturns(nil)

			result, resultErr := reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})

			Expect(repository.UpdateCallCount()).To(Equal(0))
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(resultErr).To(BeNil())
		})
		It("checks for changes to existing stateful set and does not update the cluster if there are no changes", func() {
			repository.GetCalls(func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				switch o := obj.(type) {
				case *rabbitmqv1beta1.RabbitmqCluster:
					o.Spec.Plan = "ha"
					return nil
				case *v1.Secret:
					return nil
				case *v1beta1.StatefulSet:
					return nil
				default:
					return errors.New("Test error")
				}
			})
			three := int32(3)
			statefulSet := &v1beta1.StatefulSet{
				Spec: v1beta1.StatefulSetSpec{
					Replicas: &three,
				},
			}
			foundStatefulSet := &v1beta1.StatefulSet{
				Spec: v1beta1.StatefulSetSpec{
					Replicas: &three,
				},
			}
			newSecret := &v1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rabbit",
					Namespace: "rabbit-namespace",
				},
			}
			rabbitSecret.NewReturns(newSecret, nil)
			resource := resourcegenerator.TargetResource{ResourceObject: statefulSet, EmptyResource: foundStatefulSet, Name: "", Namespace: ""}
			resources := []resourcegenerator.TargetResource{resource}
			resourceManager.ConfigureReturns(resources, nil)
			repository.SetControllerReferenceReturns(nil)
			repository.UpdateReturns(nil)

			result, resultErr := reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})

			Expect(repository.UpdateCallCount()).To(Equal(0))
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(resultErr).To(BeNil())
		})
	})
})
