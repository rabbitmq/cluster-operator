package plans

import "errors"

type Configuration struct {
	Nodes int32
}

type RabbitPlans struct {
	configurations map[string]Configuration
}

//go:generate counterfeiter . Plans
type Plans interface {
	Get(string) (Configuration, error)
}

var UnrecognisedPlanError = errors.New("Plan name is not recognised")

type UnrecognisedPlan struct{}

func (p RabbitPlans) Get(name string) (Configuration, error) {
	plan, ok := p.configurations[name]
	if ok == false {
		return Configuration{}, UnrecognisedPlanError
	}

	return plan, nil
}

func New() *RabbitPlans {
	plans := new(RabbitPlans)
	plans.configurations = map[string]Configuration{
		"single": {
			Nodes: int32(1),
		},
		"ha": {
			Nodes: int32(3),
		},
	}
	return plans
}
