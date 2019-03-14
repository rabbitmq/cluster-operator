package plans_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPlans(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Plans Suite")
}
