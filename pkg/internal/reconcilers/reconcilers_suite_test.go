package reconcilers_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestReconcilers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reconcilers Suite")
}
