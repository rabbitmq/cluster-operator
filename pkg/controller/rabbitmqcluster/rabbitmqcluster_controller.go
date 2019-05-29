/*
Copyright 2019 Pivotal.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rabbitmqcluster

import (
	"context"
	"fmt"
	"reflect"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/pkg/apis/rabbitmq/v1beta1"
	appsv1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("rabbitmqcluster-controller")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new RabbitmqCluster Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return AddController(mgr, NewReconciler(mgr))
}

// NewReconciler returns a new reconcile.Reconciler
func NewReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileRabbitmqCluster{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// AddController adds a new Controller to mgr with r as the reconcile.Reconciler
func AddController(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("rabbitmqcluster-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to RabbitmqCluster
	err = c.Watch(&source.Kind{Type: &rabbitmqv1beta1.RabbitmqCluster{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &rabbitmqv1beta1.RabbitmqCluster{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileRabbitmqCluster{}

// ReconcileRabbitmqCluster reconciles a RabbitmqCluster object
type ReconcileRabbitmqCluster struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a RabbitmqCluster object and makes changes based on the state read
// and what is in the RabbitmqCluster.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes

// Automatically generate RBAC rules to allow the Controller to read and write stateful set
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rabbitmq.pivotal.io,resources=rabbitmqclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.pivotal.io,resources=rabbitmqclusters/status,verbs=get;update;patch
func (r *ReconcileRabbitmqCluster) Reconcile(request reconcile.Request) (reconcile.Result, error) {
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
		fmt.Printf("Error reading object: %v \n", err)
		return reconcile.Result{}, err
	}

	// Create 🐰 stateful set
	single := int32(1)

	deploy := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "p-" + instance.Name,
			Namespace: instance.Namespace,
			Labels: map[string]string{
				"app": instance.Name,
			},
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: instance.Name,
			Replicas:    &single,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": instance.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": instance.Name}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "rabbitmq",
							Image: "rabbitmq:3.8-rc-management",
							Ports: []corev1.ContainerPort{
								{
									Name:          "amqp",
									ContainerPort: 5672,
								},
								{
									Name:          "http",
									ContainerPort: 15672,
								},
							},
						},
					},
				},
			},
		},
	}
	if err := controllerutil.SetControllerReference(instance, deploy, r.scheme); err != nil {
		fmt.Printf("Error setting controller reference: %v \n", err)
		return reconcile.Result{}, err
	}

	// TODO(user): Change this for the object type created by your controller
	// Check if the stateful set already exists
	found := &appsv1.StatefulSet{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating RabbitmqCluster StatefulSet", "namespace", deploy.Namespace, "name", deploy.Name)
		err = r.Create(context.TODO(), deploy)

		if err != nil {
			fmt.Printf("Error creating: %v \n", err)
		}

		return reconcile.Result{}, err
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// TODO(user): Change this for the object type created by your controller
	// Update the found object and write the result back if there are any changes
	if !reflect.DeepEqual(deploy.Spec, found.Spec) {
		found.Spec = deploy.Spec
		log.Info("Updating RabbitmqCluster StatefulSet", "namespace", deploy.Namespace, "name", deploy.Name)
		err = r.Update(context.TODO(), found)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}
