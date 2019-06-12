package resulter

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CtrlReadWrite interface {
	ctrlClient.Reader
	ctrlClient.Writer
}

type ReconcileResulter struct {
	logger     logr.Logger
	reconciler CtrlReadWrite
	ctrlScheme *runtime.Scheme
}

func New(logger logr.Logger, reconciler CtrlReadWrite, scheme *runtime.Scheme) *ReconcileResulter {
	return &ReconcileResulter{logger: logger, reconciler: reconciler, ctrlScheme: scheme}
}

func (f *ReconcileResulter) Result(req ctrl.Request) (reconcile.Result, error) {
	instance := &rabbitmqv1beta1.RabbitmqCluster{}
	err := f.reconciler.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		f.logger.Error(err, "failed getting Rabbitmq cluster object")
		return reconcile.Result{}, err
	}

	// Create 🐰 stateful set
	single := int32(1)

	statefulSet := &appsv1.StatefulSet{
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
							Env: []corev1.EnvVar{
								{
									Name:  "RABBITMQ_ENABLED_PLUGINS_FILE",
									Value: "/opt/rabbitmq-configmap/enabled_plugins",
								},
							},
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
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "rabbitmq-default-plugins",
									MountPath: "/opt/rabbitmq-configmap/",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "rabbitmq-default-plugins",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "rabbitmq-default-plugins",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(instance, statefulSet, f.ctrlScheme); err != nil {
		f.logger.Error(err, "Failed setting controller reference using StatefulSet")
		return reconcile.Result{}, err
	}

	// Check if the stateful set already exists
	found := &appsv1.StatefulSet{}
	err = f.reconciler.Get(context.TODO(), types.NamespacedName{Name: statefulSet.Name, Namespace: statefulSet.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		f.logger.Info(fmt.Sprintf("Creating RabbitmqCluster StatefulSet with namespace: %s and name: %s", statefulSet.Namespace, statefulSet.Name))
		err = f.reconciler.Create(context.TODO(), statefulSet)
		if err != nil {
			f.logger.Error(err, "Failed creating RabbitmqCluster StatefulSet")
		}

		//return reconcile.Result{}, err
	} else if err != nil {
		f.logger.Error(err, "Failed getting RabbitmqCluster StatefulSet")
		return reconcile.Result{}, err
	} else if !reflect.DeepEqual(statefulSet.Spec, found.Spec) {
		// Update the found object and write the result back if there are any changes
		// TODO at the moment we don't care what the spec looks like because we don't know what we want in the spec.
		// Once we have determined the set of properties that must exist in the spec in order to deliver the features that customers want,
		// we should do better comparison testing on the desired and actual object.
		found.Spec = statefulSet.Spec
		f.logger.Info(fmt.Sprintf("Updating RabbitmqCluster StatefulSet namespace: %s name: %s", statefulSet.Namespace, statefulSet.Name))

		err = f.reconciler.Update(context.TODO(), found)
		if err != nil {
			f.logger.Error(err, "Failed updating RabbitmqCluster StatefulSet")
			return reconcile.Result{}, err
		}
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rabbitmq-default-plugins",
			Namespace: instance.Namespace,
		},
		Data: map[string]string{
			"enabled_plugins": "[" +
				"rabbitmq_management," +
				"rabbitmq_peer_discovery_k8s," +
				"rabbitmq_federation," +
				"rabbitmq_federation_management," +
				"rabbitmq_shovel," +
				"rabbitmq_shovel_management," +
				"rabbitmq_prometheus].",
		},
	}

	if err := controllerutil.SetControllerReference(instance, configMap, f.ctrlScheme); err != nil {
		f.logger.Error(err, "Failed setting controller reference using ConfigMap")
		return reconcile.Result{}, err
	}

	// Check if the stateful set already exists
	foundConfigMap := &corev1.ConfigMap{}
	err = f.reconciler.Get(context.TODO(), types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, foundConfigMap)
	if err != nil && errors.IsNotFound(err) {
		f.logger.Info(fmt.Sprintf("Creating RabbitmqCluster ConfigMap namespace: %s name: %s", configMap.Namespace, configMap.Name))
		err = f.reconciler.Create(context.TODO(), configMap)

		if err != nil {
			f.logger.Error(err, "Failed creating RabbitmqCluster ConfigMap")
		}

		return reconcile.Result{}, err
	} else if err != nil {
		f.logger.Error(err, "Failed getting RabbitmqCluster ConfigMap object")
		return reconcile.Result{}, err
	} else if !reflect.DeepEqual(configMap.Data, foundConfigMap.Data) {
		foundConfigMap.Data = configMap.Data
		f.logger.Info(fmt.Sprintf("Updating RabbitmqCluster ConfigMap namespace: %s name: %s", configMap.Namespace, configMap.Name))
		err = f.reconciler.Update(context.TODO(), foundConfigMap)
		if err != nil {
			f.logger.Error(err, "Failed updating RabbitmqCluster ConfigMap")
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}
