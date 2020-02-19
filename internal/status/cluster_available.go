package status

import (
	"time"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterAvailableConditionManager struct {
	condition        rabbitmqv1beta1.RabbitmqClusterCondition
	serviceEndpoints *corev1.Endpoints
}

func NewClusterAvailableConditionManager(childServiceEndpoints *corev1.Endpoints) ClusterAvailableConditionManager {
	return ClusterAvailableConditionManager{
		condition:        generateCondition(rabbitmqv1beta1.ClusterAvailable),
		serviceEndpoints: childServiceEndpoints,
	}
}

func (manager *ClusterAvailableConditionManager) Condition() rabbitmqv1beta1.RabbitmqClusterCondition {
	manager.condition.LastTransitionTime = metav1.Time{
		Time: time.Unix(0, 0),
	}

	if manager.serviceEndpoints == nil {
		manager.condition.Status = corev1.ConditionFalse
		manager.condition.Reason = "CouldNotAccessServiceEndpoints"
		manager.condition.Message = "Could not verify available service endpoints"
		return manager.condition
	}

	if len(manager.serviceEndpoints.Subsets) > 0 {
		for _, subset := range manager.serviceEndpoints.Subsets {
			if len(subset.Addresses) > 0 {
				manager.condition.Status = corev1.ConditionTrue
				manager.condition.Reason = "AtLeastOneNodeAvailable"
				return manager.condition
			}
		}
	}

	manager.condition.Status = corev1.ConditionFalse
	manager.condition.Reason = "NoServiceEndpointsAvailable"
	manager.condition.Message = "The ingress service has no pod endpoints available or ready"
	return manager.condition
}
