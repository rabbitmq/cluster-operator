// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
//

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
