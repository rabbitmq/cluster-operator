package topologycontrollers

import (
	"context"
	"errors"
	"fmt"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"

	"github.com/rabbitmq/cluster-operator/internal/rabbitmqclient"
	"github.com/rabbitmq/cluster-operator/internal/topology"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups=rabbitmq.com,resources=federations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=federations/finalizers,verbs=update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=federations/status,verbs=get;update;patch

type FederationReconciler struct {
	client.Client
}

func (r *FederationReconciler) DeclareFunc(ctx context.Context, client rabbitmqclient.Client, obj rabbitmqv1beta1.TopologyResource) error {
	federation := obj.(*rabbitmqv1beta1.Federation)
	uri, err := r.getUri(ctx, federation)
	if err != nil {
		return fmt.Errorf("failed to parse federation uri secret; secret name: %s, error: %w", federation.Spec.UriSecret.Name, err)
	}
	return validateResponse(client.PutFederationUpstream(federation.Spec.Vhost, federation.Spec.Name, internal.GenerateFederationDefinition(federation, uri)))
}

func (r *FederationReconciler) getUri(ctx context.Context, federation *rabbitmqv1beta1.Federation) (string, error) {
	if federation.Spec.UriSecret == nil {
		return "", fmt.Errorf("no uri secret provided")
	}
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: federation.Spec.UriSecret.Name, Namespace: federation.Namespace}, secret); err != nil {
		return "", err
	}

	uri, ok := secret.Data["uri"]
	if !ok {
		return "", fmt.Errorf("could not find key 'uri' in secret %s", secret.Name)
	}

	return string(uri), nil
}

// deletes federation from rabbitmq server
// if server responds with '404' Not Found, it logs and does not requeue on error
func (r *FederationReconciler) DeleteFunc(ctx context.Context, client rabbitmqclient.Client, obj rabbitmqv1beta1.TopologyResource) error {
	logger := ctrl.LoggerFrom(ctx)
	federation := obj.(*rabbitmqv1beta1.Federation)
	err := validateResponseForDeletion(client.DeleteFederationUpstream(federation.Spec.Vhost, federation.Spec.Name))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find federation upstream parameter; no need to delete it", "federation", federation.Spec.Name)
	} else if err != nil {
		return err
	}
	return nil
}
