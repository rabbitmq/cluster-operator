package system_tests

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var rabbitmqHostName, rabbitmqUsername, rabbitmqPassword string

func TestSystemTests(t *testing.T) {
	rabbitmqHostName = MustHaveEnv("SERVICE_HOST")
	rabbitmqUsername = MustHaveEnv("RABBITMQ_USERNAME")
	rabbitmqPassword = MustHaveEnv("RABBITMQ_PASSWORD")

	RegisterFailHandler(Fail)
	RunSpecs(t, "SystemTests Suite")
}

func MustHaveEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		panic(fmt.Sprintf("Value '%s' not found", name))
	}
	return value
}
