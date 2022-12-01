package managedresource_test

import (
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ManagedResource Suite")
}

var testRabbitmqClusterReference = &rabbitmqv1beta1.RabbitmqClusterReference{
	Name:      "test-rabbit",
	Namespace: "example-namespace",
}
