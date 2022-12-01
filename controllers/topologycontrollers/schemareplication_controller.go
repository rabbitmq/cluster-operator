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

const SchemaReplicationParameterName = "schema_definition_sync_upstream"

// +kubebuilder:rbac:groups=rabbitmq.com,resources=schemareplications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=schemareplications/finalizers,verbs=update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=schemareplications/status,verbs=get;update;patch

type SchemaReplicationReconciler struct {
	client.Client
}

func (r *SchemaReplicationReconciler) DeclareFunc(ctx context.Context, client rabbitmqclient.Client, obj rabbitmqv1beta1.TopologyResource) error {
	replication := obj.(*rabbitmqv1beta1.SchemaReplication)
	endpoints, err := r.getUpstreamEndpoints(ctx, replication)
	if err != nil {
		return fmt.Errorf("failed to generate upstream endpoints: %w", err)
	}
	return validateResponse(client.PutGlobalParameter(SchemaReplicationParameterName, endpoints))
}

func (r *SchemaReplicationReconciler) DeleteFunc(ctx context.Context, client rabbitmqclient.Client, obj rabbitmqv1beta1.TopologyResource) error {
	logger := ctrl.LoggerFrom(ctx)
	err := validateResponseForDeletion(client.DeleteGlobalParameter(SchemaReplicationParameterName))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find global parameter; no need to delete it", "parameter", SchemaReplicationParameterName)
	} else if err != nil {
		return err
	}
	return nil
}

func (r *SchemaReplicationReconciler) getUpstreamEndpoints(ctx context.Context, replication *rabbitmqv1beta1.SchemaReplication) (internal.UpstreamEndpoints, error) {
	secret := &corev1.Secret{}
	if replication.Spec.SecretBackend.Vault != nil && replication.Spec.SecretBackend.Vault.SecretPath != "" {
		secretStoreClient, err := rabbitmqclient.SecretStoreClientProvider()
		if err != nil {
			return internal.UpstreamEndpoints{}, fmt.Errorf("unable to create a vault client connection to secret store: %w", err)
		}

		user, pass, err := secretStoreClient.ReadCredentials(replication.Spec.SecretBackend.Vault.SecretPath)
		if err != nil {
			return internal.UpstreamEndpoints{}, fmt.Errorf("unable to retrieve credentials from secret store: %w", err)
		}
		secret.Data = make(map[string][]byte)
		secret.Data["username"] = []byte(user)
		secret.Data["password"] = []byte(pass)
	} else if replication.Spec.UpstreamSecret == nil {
		return internal.UpstreamEndpoints{}, fmt.Errorf("no upstream secret or secretBackend provided")
	} else {
		if err := r.Get(ctx, types.NamespacedName{Name: replication.Spec.UpstreamSecret.Name, Namespace: replication.Namespace}, secret); err != nil {
			return internal.UpstreamEndpoints{}, err
		}
	}

	endpoints, err := internal.GenerateSchemaReplicationParameters(secret, replication.Spec.Endpoints)
	if err != nil {
		return internal.UpstreamEndpoints{}, err
	}

	return endpoints, nil
}
