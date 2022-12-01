package topologycontrollers

import (
	"context"
	"errors"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"

	"github.com/rabbitmq/cluster-operator/internal/rabbitmqclient"
	"github.com/rabbitmq/cluster-operator/internal/topology"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups=rabbitmq.com,resources=vhosts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=vhosts/finalizers,verbs=update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=vhosts/status,verbs=get;update;patch

type VhostReconciler struct {
	client.Client
}

func (r *VhostReconciler) DeclareFunc(ctx context.Context, client rabbitmqclient.Client, obj rabbitmqv1beta1.TopologyResource) error {
	vhost := obj.(*rabbitmqv1beta1.Vhost)
	return validateResponse(client.PutVhost(vhost.Spec.Name, *internal.GenerateVhostSettings(vhost)))
}

// deletes vhost from server
// if server responds with '404' Not Found, it logs and does not requeue on error
func (r *VhostReconciler) DeleteFunc(ctx context.Context, client rabbitmqclient.Client, obj rabbitmqv1beta1.TopologyResource) error {
	logger := ctrl.LoggerFrom(ctx)
	vhost := obj.(*rabbitmqv1beta1.Vhost)
	err := validateResponseForDeletion(client.DeleteVhost(vhost.Spec.Name))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find vhost in rabbitmq server; already deleted", "vhost", vhost.Spec.Name)
	} else if err != nil {
		return err
	}
	return nil
}
