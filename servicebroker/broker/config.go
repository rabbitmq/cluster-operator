package broker

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strings"

	validator "gopkg.in/go-playground/validator.v9"
	yaml "gopkg.in/yaml.v2"
)

var validate = validator.New()

func init() {
	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		return field.Tag.Get("yaml")
	})
}

type Config struct {
	Broker         Broker         `yaml:"broker" validate:"required"`
	ServiceCatalog ServiceCatalog `yaml:"service_catalog" validate:"required"`
}

type Broker struct {
	Port     int    `yaml:"port" validate:"required"`
	Username string `yaml:"username" validate:"required"`
	Password string `yaml:"password" validate:"required"`
}

type ServiceCatalog struct {
	ID          string `yaml:"id" validate:"required"`
	Name        string `yaml:"service_name" validate:"required"`
	Description string `yaml:"service_description" validate:"required"`
	Plans       []Plan `yaml:"plans" validate:"required"`
}

type Plan struct {
	ID          string `yaml:"plan_id" validate:"required"`
	Name        string `yaml:"name" validate:"required"`
	Description string `yaml:"description" validate:"required"`
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

	return config, nil
}

func ValidateConfig(config Config) error {

	if err := validate.Struct(config); err != nil {
		if errs, ok := err.(validator.ValidationErrors); ok {
			var missing []string
			for _, err := range errs {
				missing = append(missing, strings.TrimPrefix(err.Namespace(), "Config."))
			}
			return fmt.Errorf("Config file has missing fields: " + strings.Join(missing, ", "))
		}
		return err
	}

	return nil
}
