package resourcemanager

import (
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/pkg/apis/rabbitmq/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/plans"
	resourcegenerator "github.com/pivotal/rabbitmq-for-kubernetes/pkg/internal/resourcegenerator"
)

//go:generate counterfeiter . ResourceManager

type ResourceManager interface {
	Configure(*rabbitmqv1beta1.RabbitmqCluster) ([]resourcegenerator.TargetResource, error)
}

type RabbitResourceManager struct {
	generator resourcegenerator.ResourceGenerator
	plans     plans.Plans
}

func NewRabbitResourceManager(plans plans.Plans, generator resourcegenerator.ResourceGenerator) *RabbitResourceManager {
	return &RabbitResourceManager{plans: plans, generator: generator}
}

func (r *RabbitResourceManager) Configure(instance *rabbitmqv1beta1.RabbitmqCluster) ([]resourcegenerator.TargetResource, error) {

	plan, errPlan := r.plans.Get(instance.Spec.Plan)
	if errPlan != nil {
		return []resourcegenerator.TargetResource{}, errPlan
	}
	generationContext := resourcegenerator.GenerationContext{
		InstanceName: instance.Name,
		Namespace:    instance.Namespace,
		Nodes:        plan.Nodes,
	}
	return r.generator.Build(generationContext)
}
