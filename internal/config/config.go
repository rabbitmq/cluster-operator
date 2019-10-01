package config

import (
	"fmt"

	"gopkg.in/yaml.v2"
)

type serviceConfig struct {
	Type        string            `yaml:"TYPE"`
	Annotations map[string]string `yaml:"ANNOTATIONS"`
}

type persistenceConfig struct {
	StorageClassName string `yaml:"STORAGE_CLASS_NAME"`
	Storage          string `yaml:"STORAGE"`
}

type Config struct {
	Service         serviceConfig     `yaml:"SERVICE"`
	Persistence     persistenceConfig `yaml:"PERSISTENCE"`
	ImagePullSecret string            `yaml:"IMAGE_PULL_SECRET"`
	ImageUrl        string            `yaml:"IMAGE_URL"`
}

func NewConfig(configRaw []byte) (*Config, error) {
	config := &Config{}
	var err error

	err = yaml.Unmarshal(configRaw, config)

	if err != nil {
		return nil, fmt.Errorf("could not unmarshal config: %s", err)
	}

	return config, err
}
