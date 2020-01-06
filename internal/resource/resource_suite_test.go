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
func testAnnotations(annotations map[string]string) {
	ExpectWithOffset(1, annotations).To(Equal(map[string]string{"my-annotation": "i-like-this"}))
}
