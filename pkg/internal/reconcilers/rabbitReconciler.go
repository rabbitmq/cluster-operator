package reconcilers

import (
	"context"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/pkg/apis/rabbitmq/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/resourcemanager"
	cookie "github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/secret"
	"k8s.io/api/apps/v1beta1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	SetControllerReference(owner, object metav1.Object) error
}

type RabbitReconciler struct {
	Repository
	resourceManager resourcemanager.ResourceManager
	secret          cookie.Secret
}

func NewRabbitReconciler(repository Repository, resourceManager resourcemanager.ResourceManager, secret cookie.Secret) *RabbitReconciler {
	return &RabbitReconciler{
		Repository:      repository,
		resourceManager: resourceManager,
		secret:          secret,
	}
}

func (r *RabbitReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the RabbitmqCluster instance
	instance := &rabbitmqv1beta1.RabbitmqCluster{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Secret generation has been intentionally separated from other resources as it is not idempotent and should be isolated as such
	desiredSecret, secretError := r.secret.New(instance)
	if secretError != nil {
		log.Info("Error parsing secret")
		return reconcile.Result{}, secretError
	}

	if err := r.SetControllerReference(instance, desiredSecret); err != nil {
		log.Info("Error setting controller reference for Secret", "namespace", desiredSecret.Namespace, "name", desiredSecret.Name)
		return reconcile.Result{}, err
	}

	foundSecret := &v1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: desiredSecret.Name, Namespace: desiredSecret.Namespace}, foundSecret)
	if err != nil && apierrors.IsNotFound(err) {
		log.Info("Creating Secret", "namespace", desiredSecret.Namespace, "name", desiredSecret.Name)
		err = r.Create(context.TODO(), desiredSecret)
		if err != nil {
			log.Info("Error creating Secret", "namespace", desiredSecret.Namespace, "name", desiredSecret.Name)
			return reconcile.Result{}, err
		}
	} else if err != nil {
		log.Info("Error getting Secret", "namespace", desiredSecret.Namespace, "name", desiredSecret.Name)
		return reconcile.Result{}, err
	}
	resources, configureErr := r.resourceManager.Configure(instance)

	if configureErr != nil {
		log.Info("Error configuring resources")
		return reconcile.Result{}, configureErr
	}

	for _, resource := range resources {
		if err := r.SetControllerReference(instance, resource.ResourceObject.(metav1.Object)); err != nil {
			log.Info("Error setting controller reference for "+resource.ResourceObject.GetObjectKind().GroupVersionKind().Kind, "namespace", resource.Namespace, "name", resource.Name)
			return reconcile.Result{}, err
		}

		found := resource.EmptyResource
		err = r.Get(context.TODO(), types.NamespacedName{Name: resource.Name, Namespace: resource.Namespace}, found)
		if err != nil && apierrors.IsNotFound(err) {
			log.Info("Creating "+resource.ResourceObject.GetObjectKind().GroupVersionKind().Kind, "namespace", resource.Namespace, "name", resource.Name)
			err = r.Create(context.TODO(), resource.ResourceObject)
			if err != nil {
				log.Info("Error creating "+resource.ResourceObject.GetObjectKind().GroupVersionKind().Kind, "namespace", resource.Namespace, "name", resource.Name)
				return reconcile.Result{}, err
			}
		} else if err != nil {
			log.Info("Error getting "+resource.ResourceObject.GetObjectKind().GroupVersionKind().Kind, "namespace", resource.Namespace, "name", resource.Name)
			return reconcile.Result{}, err
		} else {
			switch o := resource.ResourceObject.(type) {
			case *v1beta1.StatefulSet:
				foundStatefulSet := resource.EmptyResource.(*v1beta1.StatefulSet)
				if *o.Spec.Replicas != *foundStatefulSet.Spec.Replicas {
					*foundStatefulSet.Spec.Replicas = *o.Spec.Replicas
					log.Info("Updating "+resource.ResourceObject.GetObjectKind().GroupVersionKind().Kind, "namespace", resource.Namespace, "name", resource.Name)
					if err := r.Update(context.TODO(), foundStatefulSet); err != nil {
						log.Info("Error updating "+resource.ResourceObject.GetObjectKind().GroupVersionKind().Kind, "namespace", resource.Namespace, "name", resource.Name)
						return reconcile.Result{}, err
					}
				}
			}
		}
	}

	return reconcile.Result{}, nil
}
