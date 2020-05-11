// Copyright (c) 2020 VMware, Inc. or its affiliates.  All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

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
