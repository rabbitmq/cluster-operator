package resourcemanager

import (
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/pkg/apis/rabbitmq/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/plans"
	. "github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/resourcegenerator"
)

//go:generate counterfeiter . ResourceManager

type ResourceManager interface {
	Configure(*rabbitmqv1beta1.RabbitmqCluster, plans.Plans, ResourceGenerator) ([]TargetResource, error)
}

type RabbitResourceManager struct{}

func (r *RabbitResourceManager) Configure(instance *rabbitmqv1beta1.RabbitmqCluster, plans plans.Plans, generator ResourceGenerator) ([]TargetResource, error) {

	plan, errPlan := plans.Get(instance.Spec.Plan)
	if errPlan != nil {
		return []TargetResource{}, errPlan
	}
	generationContext := GenerationContext{
		InstanceName: instance.Name,
		Namespace:    instance.Namespace,
		Nodes:        plan.Nodes,
	}
	return generator.Build(generationContext)
}
