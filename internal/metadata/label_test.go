// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
//

package metadata_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rabbitmq/cluster-operator/v2/internal/metadata"
)

var _ = Describe("ValidateStatefulSetSelector", func() {
	It("allows a nil selector with no template label overrides", func() {
		Expect(metadata.ValidateStatefulSetSelector(nil, "my-instance", nil)).To(Succeed())
	})

	It("rejects a nil selector when a template label override breaks the default selector match", func() {
		err := metadata.ValidateStatefulSetSelector(nil, "my-instance", map[string]string{"app.kubernetes.io/name": "renamed"})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("selector does not match"))
	})

	It("allows a nil selector when template label overrides don't touch the default selector label", func() {
		Expect(metadata.ValidateStatefulSetSelector(nil, "my-instance", map[string]string{"extra": "label"})).To(Succeed())
	})

	It("rejects an explicit empty selector even though it trivially matches everything", func() {
		err := metadata.ValidateStatefulSetSelector(&metav1.LabelSelector{}, "my-instance", nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("must not be empty"))
	})

	It("allows an explicit selector that matches the operator's default labels", func() {
		selector := &metav1.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/name": "my-instance"}}
		Expect(metadata.ValidateStatefulSetSelector(selector, "my-instance", nil)).To(Succeed())
	})

	It("allows an explicit selector matched by an added template label override", func() {
		selector := &metav1.LabelSelector{MatchLabels: map[string]string{"my-label": "my-value"}}
		Expect(metadata.ValidateStatefulSetSelector(selector, "my-instance", map[string]string{"my-label": "my-value"})).To(Succeed())
	})

	It("rejects an explicit selector that doesn't match the template labels", func() {
		selector := &metav1.LabelSelector{MatchLabels: map[string]string{"my-label": "my-value"}}
		err := metadata.ValidateStatefulSetSelector(selector, "my-instance", nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("selector does not match"))
	})
})
