package topologycontrollers

// common error messages shared across controllers
const (
	failedStatusUpdate         = "failed to update object status"
	failedMarshalSpec          = "failed to marshal spec"
	failedGenerateRabbitClient = "failed to generate http rabbitClient"
	failedParseClusterRef      = "failed to retrieve cluster from reference"
	failedRetrieveSysCertPool  = "failed to retrieve system trusted certs"
	noSuchRabbitDeletion       = "RabbitmqCluster is already gone: cannot find its connection secret"
)

// names for each of the controllers
const (
	VhostControllerName             = "vhost-controller"
	QueueControllerName             = "queue-controller"
	ExchangeControllerName          = "exchange-controller"
	BindingControllerName           = "binding-controller"
	UserControllerName              = "user-controller"
	PolicyControllerName            = "policy-controller"
	PermissionControllerName        = "permission-controller"
	SchemaReplicationControllerName = "schema-replication-controller"
	FederationControllerName        = "federation-controller"
	ShovelControllerName            = "shovel-controller"
	SuperStreamControllerName       = "super-stream-controller"
	TopicPermissionControllerName   = "topic-permission-controller"
)

// names for environment variables
const (
	KubernetesInternalDomainEnvVar = "MESSAGING_DOMAIN_NAME"
	OperatorNamespaceEnvVar        = "OPERATOR_NAMESPACE"
	EnableWebhooksEnvVar           = "ENABLE_WEBHOOKS"
	ControllerSyncPeriodEnvVar     = "SYNC_PERIOD"
)
