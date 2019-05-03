package rabbithutch_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRabbithutch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rabbithutch Suite")
}
