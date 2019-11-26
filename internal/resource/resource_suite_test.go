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
	defaultMemoryRequest string = "2Gi"
	defaultCPURequest    string = "1000m"
	defaultMemoryLimit   string = "2Gi"
	defaultCPULimit      string = "2000m"
)
