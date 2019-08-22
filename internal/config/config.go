package config

import (
	"fmt"
	"gopkg.in/yaml.v2"
)

type ServiceConfig struct {
	Type        string            `yaml:"TYPE"`
	Annotations map[string]string `yaml:"ANNOTATIONS"`
}

func NewServiceConfig(configRaw []byte) (*ServiceConfig, error) {
	config := &ServiceConfig{}
	var err error

	err = yaml.Unmarshal(configRaw, config)

	if err != nil {
		return nil, fmt.Errorf("could not unmarshal config: %s", err)
	}

	return config, err
}
