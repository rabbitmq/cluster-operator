/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package topologycontrollers

import (
	"context"
	"fmt"
	"github.com/rabbitmq/cluster-operator/internal/rabbitmqclient"
	"strconv"

	"github.com/go-logr/logr"
	rabbitmqv1alpha1 "github.com/rabbitmq/cluster-operator/api/v1alpha1"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/topology/managedresource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	clientretry "k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// SuperStreamReconciler reconciles a RabbitMQ Super Stream, and any resources it comprises of
type SuperStreamReconciler struct {
	client.Client
	Log                     logr.Logger
	Scheme                  *runtime.Scheme
	Recorder                record.EventRecorder
	RabbitmqClientFactory   rabbitmqclient.Factory
	KubernetesClusterDomain string
}

// +kubebuilder:rbac:groups=rabbitmq.com,resources=exchanges,verbs=get;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=queues,verbs=get;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=bindings,verbs=get;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=superstreams,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=superstreams/finalizers,verbs=update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=superstreams/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters/status,verbs=get
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;create;patch

func (r *SuperStreamReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	superStream := &rabbitmqv1alpha1.SuperStream{}
	if err := r.Get(ctx, req.NamespacedName, superStream); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	rmqClusterRef, err := r.getRabbitmqClusterReference(ctx, superStream.Spec.RabbitmqClusterReference, superStream.Namespace)
	if err != nil {
		return handleRMQReferenceParseError(ctx, r.Client, r.Recorder, superStream, &superStream.Status.Conditions, err)
	}

	logger.Info("Start reconciling")

	if superStream.Spec.Partitions < len(superStream.Status.Partitions) {
		// This would constitute a scale down, which may result in data loss.
		err := fmt.Errorf(
			"SuperStreams cannot be scaled down: an attempt was made to scale from %d partitions to %d",
			len(superStream.Status.Partitions),
			superStream.Spec.Partitions,
		)
		msg := fmt.Sprintf("SuperStream %s failed to reconcile", superStream.Name)
		logger.Error(err, msg)
		r.Recorder.Event(superStream, corev1.EventTypeWarning, "FailedScaleDown", err.Error())
		if writerErr := r.SetReconcileSuccess(ctx, superStream, rabbitmqv1beta1.NotReady(msg, superStream.Status.Conditions)); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate, "status", superStream.Status)
		}
		return reconcile.Result{}, nil
	}

	var routingKeys []string
	if len(superStream.Spec.RoutingKeys) == 0 {
		routingKeys = r.generateRoutingKeys(superStream)
	} else if len(superStream.Spec.RoutingKeys) != superStream.Spec.Partitions {
		err := fmt.Errorf(
			"expected number of routing keys (%d) to match number of partitions (%d)",
			len(superStream.Spec.RoutingKeys),
			superStream.Spec.Partitions,
		)
		msg := fmt.Sprintf("SuperStream %s failed to reconcile", superStream.Name)
		logger.Error(err, msg)
		if writerErr := r.SetReconcileSuccess(ctx, superStream, rabbitmqv1beta1.NotReady(msg, superStream.Status.Conditions)); writerErr != nil {
			logger.Error(writerErr, failedStatusUpdate, "status", superStream.Status)
		}
		return reconcile.Result{}, err
	} else {
		routingKeys = superStream.Spec.RoutingKeys
	}

	// Each SuperStream generates, for n partitions, 1 exchange, n streams and n bindings
	managedResourceBuilder := managedresource.Builder{
		ObjectOwner: superStream,
		Scheme:      r.Scheme,
	}

	builders := []managedresource.ResourceBuilder{managedResourceBuilder.SuperStreamExchange(superStream.Spec.Vhost, rmqClusterRef)}
	for index, routingKey := range routingKeys {
		builders = append(
			builders,
			managedResourceBuilder.SuperStreamPartition(index, routingKey, superStream.Spec.Vhost, rmqClusterRef),
			managedResourceBuilder.SuperStreamBinding(index, routingKey, superStream.Spec.Vhost, rmqClusterRef),
		)
	}

	var partitionQueueNames []string
	for _, builder := range builders {
		resource, err := builder.Build()
		if err != nil {
			return ctrl.Result{}, err
		}

		err = clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
			var apiError error
			_, apiError = controllerutil.CreateOrUpdate(ctx, r.Client, resource, func() error {
				return builder.Update(resource)
			})
			return apiError
		})
		if err != nil {
			msg := fmt.Sprintf("FailedReconcile%s", builder.ResourceType())
			if writerErr := r.SetReconcileSuccess(ctx, superStream, rabbitmqv1beta1.NotReady(msg, superStream.Status.Conditions)); writerErr != nil {
				logger.Error(writerErr, failedStatusUpdate, "status", superStream.Status)
			}
			return ctrl.Result{}, err
		}

		if builder.ResourceType() == "Partition" {
			partition := resource.(*rabbitmqv1beta1.Queue)
			partitionQueueNames = append(partitionQueueNames, partition.Spec.Name)
		}
	}

	superStream.Status.Partitions = partitionQueueNames
	if err := clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, superStream)
	}); err != nil {
		logger.Error(err, failedStatusUpdate)
	}

	if err := r.SetReconcileSuccess(ctx, superStream, rabbitmqv1beta1.Ready(superStream.Status.Conditions)); err != nil {
		logger.Error(err, failedStatusUpdate)
	}

	logger.Info("Finished reconciling")

	return ctrl.Result{}, nil
}

func (r *SuperStreamReconciler) getRabbitmqClusterReference(ctx context.Context, rmq rabbitmqv1beta1.RabbitmqClusterReference, requestNamespace string) (*rabbitmqv1beta1.RabbitmqClusterReference, error) {
	var namespace string
	if rmq.Namespace == "" {
		namespace = requestNamespace
	} else {
		namespace = rmq.Namespace
	}

	cluster := &rabbitmqv1beta1.RabbitmqCluster{}
	if err := r.Get(ctx, types.NamespacedName{Name: rmq.Name, Namespace: namespace}, cluster); err != nil {
		return nil, fmt.Errorf("failed to get cluster from reference: %s Error: %w", err, rabbitmqclient.NoSuchRabbitmqClusterError)
	}

	if !rabbitmqclient.AllowedNamespace(rmq, requestNamespace, cluster) {
		return nil, rabbitmqclient.ResourceNotAllowedError
	}

	return &rabbitmqv1beta1.RabbitmqClusterReference{
		Name:      rmq.Name,
		Namespace: namespace,
	}, nil
}

func (r *SuperStreamReconciler) generateRoutingKeys(superStream *rabbitmqv1alpha1.SuperStream) (routingKeys []string) {
	for i := 0; i < superStream.Spec.Partitions; i++ {
		routingKeys = append(routingKeys, strconv.Itoa(i))
	}
	return routingKeys
}

func (r *SuperStreamReconciler) SetReconcileSuccess(ctx context.Context, superStream *rabbitmqv1alpha1.SuperStream, condition rabbitmqv1beta1.Condition) error {
	superStream.Status.Conditions = []rabbitmqv1beta1.Condition{condition}
	superStream.Status.ObservedGeneration = superStream.GetGeneration()
	return clientretry.RetryOnConflict(clientretry.DefaultRetry, func() error {
		return r.Status().Update(ctx, superStream)
	})
}

func (r *SuperStreamReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rabbitmqv1alpha1.SuperStream{}).
		Owns(&rabbitmqv1beta1.Exchange{}).
		Owns(&rabbitmqv1beta1.Binding{}).
		Owns(&rabbitmqv1beta1.Queue{}).
		Complete(r)
}
