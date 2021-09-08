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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/internal/resource"
	. "github.com/rabbitmq/cluster-operator/internal/resource"
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

			builder *resource.RabbitmqResourceBuilder
			scheme  *runtime.Scheme
		)

		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
			Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())
			builder = &resource.RabbitmqResourceBuilder{
				Instance: &instance,
				Scheme:   scheme,
			}
		})

		It("returns the required resource builders in the expected order", func() {
			resourceBuilders := builder.ResourceBuilders()

			Expect(resourceBuilders).To(HaveLen(10))

			expectedBuildersInOrder := []ResourceBuilder{
				&HeadlessServiceBuilder{},
				&ServiceBuilder{},
				&ErlangCookieBuilder{},
				&DefaultUserSecretBuilder{},
				&RabbitmqPluginsConfigMapBuilder{},
				&ServerConfigMapBuilder{},
				&ServiceAccountBuilder{},
				&RoleBuilder{},
				&RoleBindingBuilder{},
				&StatefulSetBuilder{},
			}

			for i, resourceBuilder := range resourceBuilders {
				Expect(resourceBuilder).To(BeAssignableToTypeOf(expectedBuildersInOrder[i]))
			}
		})

		When("vault is enabled", func() {
			BeforeEach(func() {
				builder.Instance.Spec.SecretBackend.Vault.Role = "myrole"
				builder.Instance.Spec.SecretBackend.Vault.DefaultUserSecretPath = "somepath"

			})
			It("returns all resource builders except defaultUser when vault is enabled", func() {
				resourceBuilders := builder.ResourceBuilders()
				Expect(resourceBuilders).NotTo(ContainElement(BeAssignableToTypeOf(&DefaultUserSecretBuilder{})))
			})
		})

	})
})
