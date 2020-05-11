// Copyright (c) 2020 VMware, Inc. or its affiliates.  All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("RabbitmqResourceBuilder", func() {
	Context("ResourceBuilders", func() {
		var (
			instance = rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test",
					Namespace: "namespace",
				},
			}

			rabbitmqCluster *resource.RabbitmqResourceBuilder
			scheme          *runtime.Scheme
		)

		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
			Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())
			rabbitmqCluster = &resource.RabbitmqResourceBuilder{
				Instance: &instance,
				Scheme:   scheme,
			}
		})

		It("returns the required resource builders in the expected order", func() {
			resourceBuilders, err := rabbitmqCluster.ResourceBuilders()
			Expect(err).NotTo(HaveOccurred())

			Expect(len(resourceBuilders)).To(Equal(9))

			var ok bool
			_, ok = resourceBuilders[0].(*resource.HeadlessServiceBuilder)
			Expect(ok).Should(BeTrue())
			_, ok = resourceBuilders[1].(*resource.IngressServiceBuilder)
			Expect(ok).Should(BeTrue())
			_, ok = resourceBuilders[2].(*resource.ErlangCookieBuilder)
			Expect(ok).Should(BeTrue())
			_, ok = resourceBuilders[3].(*resource.AdminSecretBuilder)
			Expect(ok).Should(BeTrue())
			_, ok = resourceBuilders[4].(*resource.ServerConfigMapBuilder)
			Expect(ok).Should(BeTrue())
			_, ok = resourceBuilders[5].(*resource.ServiceAccountBuilder)
			Expect(ok).Should(BeTrue())
			_, ok = resourceBuilders[6].(*resource.RoleBuilder)
			Expect(ok).Should(BeTrue())
			_, ok = resourceBuilders[7].(*resource.RoleBindingBuilder)
			Expect(ok).Should(BeTrue())
			_, ok = resourceBuilders[8].(*resource.StatefulSetBuilder)
			Expect(ok).Should(BeTrue())
		})
	})
})
