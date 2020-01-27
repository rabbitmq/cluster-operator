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

func testLabels(labels map[string]string) {
	ExpectWithOffset(1, labels).To(SatisfyAll(
		HaveKeyWithValue("foo", "bar"),
		HaveKeyWithValue("rabbitmq", "is-great"),
		HaveKeyWithValue("foo/app.kubernetes.io", "edgecase"),
		Not(HaveKey("app.kubernetes.io/foo")),
	))
}
