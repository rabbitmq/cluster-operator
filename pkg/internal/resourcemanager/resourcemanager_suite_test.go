package resourcemanager_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestResourcemanager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resourcemanager Suite")
}
