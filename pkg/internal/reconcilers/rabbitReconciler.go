package reconcilers

import (
	"context"
	"reflect"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/pkg/apis/rabbitmq/v1beta1"
	generator "github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/resourcegenerator"
	"k8s.io/api/apps/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("controller")

//go:generate counterfeiter . Repository

type Repository interface {
	Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error
	Create(ctx context.Context, obj runtime.Object) error
	Update(ctx context.Context, obj runtime.Object) error
	SetControllerReference(owner, object v1.Object) error
}

type RabbitReconciler struct {
	Repository
	Generator generator.ResourceGenerator
}

func NewRabbitReconciler(repository Repository, generator generator.ResourceGenerator) *RabbitReconciler {
	return &RabbitReconciler{
		Repository: repository,
		Generator:  generator,
	}
}

func (r *RabbitReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the RabbitmqCluster instance
	instance := &rabbitmqv1beta1.RabbitmqCluster{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	generationContext := generator.GenerationContext{
		InstanceName: instance.Name,
		Namespace:    instance.Namespace,
		Nodes:        instance.Spec.Nodes,
	}
	resources, err := r.Generator.Build(generationContext)
	if err != nil {
		return reconcile.Result{}, err
	}

	for _, resource := range resources {
		if err := r.SetControllerReference(instance, resource.ResourceObject.(v1.Object)); err != nil {
			return reconcile.Result{}, err
		}

		found := resource.EmptyResource
		err = r.Get(context.TODO(), types.NamespacedName{Name: resource.Name, Namespace: resource.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			log.Info("Creating "+resource.ResourceObject.GetObjectKind().GroupVersionKind().Kind, "namespace", resource.Namespace, "name", resource.Name)
			err = r.Create(context.TODO(), resource.ResourceObject)
			if err != nil {
				return reconcile.Result{}, err
			}
		} else if err != nil {
			return reconcile.Result{}, err
		} else {
			switch o := resource.ResourceObject.(type) {
			case *v1beta1.StatefulSet:
				foundStatefulSet := resource.EmptyResource.(*v1beta1.StatefulSet)
				if !reflect.DeepEqual(o.Spec, foundStatefulSet.Spec) {
					foundStatefulSet.Spec = o.Spec
					log.Info("Updating "+resource.ResourceObject.GetObjectKind().GroupVersionKind().Kind, "namespace", resource.Namespace, "name", resource.Name)
					if err := r.Update(context.TODO(), foundStatefulSet); err != nil {
						return reconcile.Result{}, err
					}
				}
			}
		}
	}

	return reconcile.Result{}, nil
}
