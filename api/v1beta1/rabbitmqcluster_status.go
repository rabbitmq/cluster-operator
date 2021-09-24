package v1beta1

import (
	"github.com/rabbitmq/cluster-operator/internal/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Status presents the observed state of RabbitmqCluster
type RabbitmqClusterStatus struct {
	// Set of Conditions describing the current state of the RabbitmqCluster
	Conditions []status.RabbitmqClusterCondition `json:"conditions"`

	// Identifying information on internal resources
	DefaultUser *RabbitmqClusterDefaultUser `json:"defaultUser,omitempty"`

	// Binding exposes a secret containing the binding information for this
	// RabbitmqCluster. It implements the service binding Provisioned Service
	// duck type. See: https://github.com/servicebinding/spec#provisioned-service
	Binding *corev1.LocalObjectReference `json:"binding,omitempty"`

	// observedGeneration is the most recent successful generation observed for this RabbitmqCluster. It corresponds to the
	// RabbitmqCluster's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// Contains references to resources created with the RabbitmqCluster resource.
type RabbitmqClusterDefaultUser struct {
	// Reference to the Kubernetes Secret containing the credentials of the default
	// user.
	SecretReference *RabbitmqClusterSecretReference `json:"secretReference,omitempty"`
	// Reference to the Kubernetes Service serving the cluster.
	ServiceReference *RabbitmqClusterServiceReference `json:"serviceReference,omitempty"`
}

// Reference to the Kubernetes Secret containing the credentials of the default user.
type RabbitmqClusterSecretReference struct {
	// Name of the Secret containing the default user credentials
	Name string `json:"name"`
	// Namespace of the Secret containing the default user credentials
	Namespace string `json:"namespace"`
	// Key-value pairs in the Secret corresponding to `username`, `password`, `host`, and `port`
	Keys map[string]string `json:"keys"`
}

// Reference to the Kubernetes Service serving the cluster.
type RabbitmqClusterServiceReference struct {
	// Name of the Service serving the cluster
	Name string `json:"name"`
	// Namespace of the Service serving the cluster
	Namespace string `json:"namespace"`
}

func (clusterStatus *RabbitmqClusterStatus) SetConditions(resources []runtime.Object) {
	var oldAllPodsReadyCondition *status.RabbitmqClusterCondition
	var oldClusterAvailableCondition *status.RabbitmqClusterCondition
	var oldNoWarningsCondition *status.RabbitmqClusterCondition
	var oldReconcileCondition *status.RabbitmqClusterCondition

	for _, condition := range clusterStatus.Conditions {
		switch condition.Type {
		case status.AllReplicasReady:
			oldAllPodsReadyCondition = condition.DeepCopy()
		case status.ClusterAvailable:
			oldClusterAvailableCondition = condition.DeepCopy()
		case status.NoWarnings:
			oldNoWarningsCondition = condition.DeepCopy()
		case status.ReconcileSuccess:
			oldReconcileCondition = condition.DeepCopy()
		}
	}

	allReplicasReadyCond := status.AllReplicasReadyCondition(resources, oldAllPodsReadyCondition)
	clusterAvailableCond := status.ClusterAvailableCondition(resources, oldClusterAvailableCondition)
	noWarningsCond := status.NoWarningsCondition(resources, oldNoWarningsCondition)

	var reconciledCondition status.RabbitmqClusterCondition
	if oldReconcileCondition != nil {
		reconciledCondition = *oldReconcileCondition
	} else {
		reconciledCondition = status.ReconcileSuccessCondition(corev1.ConditionUnknown, "Initialising", "")
	}

	clusterStatus.Conditions = []status.RabbitmqClusterCondition{
		allReplicasReadyCond,
		clusterAvailableCond,
		noWarningsCond,
		reconciledCondition,
	}
}

func (clusterStatus *RabbitmqClusterStatus) SetCondition(condType status.RabbitmqClusterConditionType,
	condStatus corev1.ConditionStatus, reason string, messages ...string) {
	for i := range clusterStatus.Conditions {
		if clusterStatus.Conditions[i].Type == condType {
			clusterStatus.Conditions[i].UpdateState(condStatus)
			clusterStatus.Conditions[i].UpdateReason(reason, messages...)
			break
		}
	}
}
