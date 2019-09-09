package resource_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resource Suite")
}

const (
	StatefulSetSuffix string = "-rabbitmq-server"
	SecretSuffix      string = "-rabbitmq-admin"
	ServiceSuffix     string = "-rabbitmq-ingress"
	ConfigMapSuffix   string = "-rabbitmq-plugins"
)
