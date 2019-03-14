package plans

import "errors"

type Configuration struct {
	Nodes int32
}

type Plans struct {
	configurations map[string]Configuration
}

var UnrecognisedPlanError = errors.New("Plan name is no recognised")

type UnrecognisedPlan struct{}

func (p Plans) Get(name string) (Configuration, error) {
	plan, ok := p.configurations[name]
	if ok == false {
		return Configuration{}, UnrecognisedPlanError
	}

	return plan, nil
}

func New() *Plans {
	plans := new(Plans)
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
