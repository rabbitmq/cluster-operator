package reconcilers_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/reconcilers"
	"github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/reconcilers/reconcilersfakes"
	. "github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/resourcegenerator"
	"github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/resourcegenerator/resourcegeneratorfakes"
	"k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Rabbitreconciler", func() {
	var (
		reconciler      *RabbitReconciler
		repository      *reconcilersfakes.FakeRepository
		generator       *resourcegeneratorfakes.FakeResourceGenerator
		notFoundError   *apierrors.StatusError
		badRequestError *apierrors.StatusError
	)

	Context("Reconcile", func() {
		BeforeEach(func() {
			repository = new(reconcilersfakes.FakeRepository)
			generator = new(resourcegeneratorfakes.FakeResourceGenerator)
			reconciler = NewRabbitReconciler(repository, generator)

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
		It("returns an empty object and an error when kustomize fails", func() {
			repository.GetReturns(nil)
			err := errors.New("whatever")
			generator.BuildReturns(make([]TargetResource, 0), err)
			result, resultErr := reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})

			Expect(result).To(Equal(reconcile.Result{}))
			Expect(resultErr).To(Equal(err))
		})
		It("returns an empty object and an error when referencing a resource fails", func() {
			repository.GetReturns(nil)
			resource := TargetResource{ResourceObject: &v1.Service{}, EmptyResource: &v1.Service{}, Name: "", Namespace: ""}
			resources := []TargetResource{resource}
			generator.BuildReturns(resources, nil)
			err := errors.New("referencing failed")
			repository.SetControllerReferenceReturns(err)
			result, resultErr := reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})

			Expect(result).To(Equal(reconcile.Result{}))
			Expect(resultErr).To(Equal(err))
		})
		It("creates the resource if it is not found", func() {
			repository.GetReturnsOnCall(0, nil)
			resource := TargetResource{ResourceObject: &v1.Service{}, EmptyResource: &v1.Service{}, Name: "", Namespace: ""}
			resources := []TargetResource{resource}
			generator.BuildReturns(resources, nil)
			repository.SetControllerReferenceReturns(nil)
			repository.GetReturnsOnCall(1, notFoundError)

			reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})
			ctx, resourceObject := repository.CreateArgsForCall(0)

			Expect(repository.CreateCallCount()).To(Equal(1))
			Expect(ctx).To(Equal(context.TODO()))
			Expect(resourceObject).To(Equal(resource.ResourceObject))
		})
		It("creates multiple resources if they are not found", func() {
			repository.GetReturnsOnCall(0, nil)
			resource1 := TargetResource{ResourceObject: &v1.Service{}, EmptyResource: &v1.Service{}, Name: "", Namespace: ""}
			resource2 := TargetResource{ResourceObject: &v1beta1.StatefulSet{}, EmptyResource: &v1beta1.StatefulSet{}, Name: "", Namespace: ""}
			resources := []TargetResource{resource1, resource2}
			generator.BuildReturns(resources, nil)
			repository.SetControllerReferenceReturns(nil)
			repository.GetReturnsOnCall(1, notFoundError)
			repository.GetReturnsOnCall(2, notFoundError)

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
			repository.GetReturnsOnCall(0, nil)
			resource := TargetResource{ResourceObject: &v1.Service{}, EmptyResource: &v1.Service{}, Name: "", Namespace: ""}
			resources := []TargetResource{resource}
			generator.BuildReturns(resources, nil)
			repository.SetControllerReferenceReturns(nil)
			repository.GetReturnsOnCall(1, notFoundError)
			createError := errors.New("fake error")
			repository.CreateReturns(createError)

			result, resultErr := reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})

			Expect(result).To(Equal(reconcile.Result{}))
			Expect(resultErr).To(Equal(createError))
		})
		It("returns an empty object and an error in case of unexpected error when loading the resource", func() {
			repository.GetReturnsOnCall(0, nil)
			resource := TargetResource{ResourceObject: &v1.Service{}, EmptyResource: &v1.Service{}, Name: "", Namespace: ""}
			resources := []TargetResource{resource}
			generator.BuildReturns(resources, nil)
			repository.SetControllerReferenceReturns(nil)
			repository.GetReturnsOnCall(1, badRequestError)

			result, resultErr := reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})

			Expect(result).To(Equal(reconcile.Result{}))
			Expect(resultErr).To(Equal(badRequestError))
		})
		It("checks for changes to existing stateful set and updates the cluster", func() {
			repository.GetReturnsOnCall(0, nil)
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
			resource := TargetResource{ResourceObject: statefulSet, EmptyResource: foundStatefulSet, Name: "", Namespace: ""}
			resources := []TargetResource{resource}
			generator.BuildReturns(resources, nil)
			repository.SetControllerReferenceReturns(nil)
			repository.GetReturnsOnCall(1, nil)
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
			repository.GetReturnsOnCall(0, nil)
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
			resource := TargetResource{ResourceObject: statefulSet, EmptyResource: foundStatefulSet, Name: "", Namespace: ""}
			resources := []TargetResource{resource}
			generator.BuildReturns(resources, nil)
			repository.SetControllerReferenceReturns(nil)
			repository.GetReturnsOnCall(1, notFoundError)

			result, resultErr := reconciler.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "rabbit", Namespace: "default"},
			})

			Expect(repository.UpdateCallCount()).To(Equal(0))
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(resultErr).To(BeNil())
		})
		It("checks for changes to existing stateful set and does not update the cluster if there are no changes", func() {
			repository.GetReturnsOnCall(0, nil)
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
			resource := TargetResource{ResourceObject: statefulSet, EmptyResource: foundStatefulSet, Name: "", Namespace: ""}
			resources := []TargetResource{resource}
			generator.BuildReturns(resources, nil)
			repository.SetControllerReferenceReturns(nil)
			repository.GetReturnsOnCall(1, nil)
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
