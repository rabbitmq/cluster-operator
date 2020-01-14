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

func testLabels(labels map[string]string) {
	ExpectWithOffset(1, labels).To(SatisfyAll(
		HaveKeyWithValue("foo", "bar"),
		HaveKeyWithValue("rabbitmq", "is-great"),
		HaveKeyWithValue("foo/app.kubernetes.io", "edgecase"),
		Not(HaveKey("app.kubernetes.io/foo")),
	))
}
func testAnnotations(actualAnnotations, expectedAnnotations map[string]string) {
	for k, v := range expectedAnnotations {
		ExpectWithOffset(1, actualAnnotations).To(HaveKeyWithValue(k, v))
	}
}
