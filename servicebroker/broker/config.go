package broker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	ServiceConfig  ServiceConfig  `yaml:"service"`
	RabbitmqConfig RabbitmqConfig `yaml:"rabbitmq"`
}

type ServiceConfig struct {
	UUID                string `yaml:"uuid"`
	Name                string `yaml:"name"`
	OfferingDescription string `yaml:"offering_description"`
	Username            string `yaml:"username"`
	Password            string `yaml:"password"`
	PlanUuid            string `yaml:"plan_uuid"`
	DisplayName         string `yaml:"display_name"`
	IconImage           string `yaml:"icon_image"`
	LongDescription     string `yaml:"long_description"`
	ProviderDisplayName string `yaml:"provider_display_name"`
	DocumentationUrl    string `yaml:"documentation_url"`
	SupportUrl          string `yaml:"support_url"`
}

type RabbitmqConfig struct {
	Hosts            []string            `yaml:"hosts"`
	DnsHost          string              `yaml:"dns_host"`
	ManagementDomain string              `yaml:"management_domain"`
	Administrator    RabbitmqCredentials `yaml:"administrator"`
	Policy           RabbitmqPolicy      `yaml:"operator_set_policy"`
}

type RabbitmqCredentials struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type RabbitmqPolicy struct {
	Enabled           bool                   `yaml:"enabled"`
	Name              string                 `yaml:"policy_name"`
	Priority          int                    `yaml:"policy_priority"`
	EncodedDefinition string                 `yaml:"policy_definition"`
	Definition        map[string]interface{} `yaml:"definition"`
}

func ReadConfig(path string) (Config, error) {
	configBytes, err := ioutil.ReadFile(filepath.FromSlash(path))
	if err != nil {
		return Config{}, err
	}

	config := Config{}
	if err = yaml.Unmarshal(configBytes, &config); err != nil {
		return Config{}, err
	}

	if err := ValidateConfig(config); err != nil {
		return Config{}, err
	}

	if err := json.Unmarshal([]byte(config.RabbitmqConfig.Policy.EncodedDefinition), &config.RabbitmqConfig.Policy.Definition); err != nil {
		return Config{}, err
	}

	return config, nil
}

func ValidateConfig(config Config) error {
	if config.ServiceConfig.UUID == "" {
		return fmt.Errorf("uuid is not set")
	}
	if config.ServiceConfig.Name == "" {
		return fmt.Errorf("service name is not set")
	}
	if config.ServiceConfig.Username == "" {
		return fmt.Errorf("service username is not set")
	}
	if config.ServiceConfig.Password == "" {
		return fmt.Errorf("service password is not set")
	}
	if config.ServiceConfig.PlanUuid == "" {
		return fmt.Errorf("plan uuid is not set")
	}
	if len(config.RabbitmqConfig.Hosts) < 1 {
		return fmt.Errorf("no rabbitmq hosts were set")
	}
	if config.RabbitmqConfig.Administrator.Username == "" {
		return fmt.Errorf("administrator username is not set")
	}
	if config.RabbitmqConfig.Administrator.Password == "" {
		return fmt.Errorf("administrator password is not set")
	}

	return nil
}
