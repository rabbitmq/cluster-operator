// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance wit the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
//

package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	. "github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
)

var _ = Describe("RabbitMQPlugins", func() {

	Context("DesiredPlugins", func() {
		When("AdditionalPlugins is empty", func() {
			It("returns list of required plugins", func() {
				plugins := NewRabbitMQPlugins(nil)
				Expect(plugins.DesiredPlugins()).To(ConsistOf([]string{"rabbitmq_peer_discovery_k8s", "rabbitmq_prometheus", "rabbitmq_management"}))
			})
		})

		When("AdditionalPlugins are provided", func() {
			It("returns a concatenated list of plugins", func() {
				morePlugins := []rabbitmqv1beta1.Plugin{"rabbitmq_shovel", "my_great_plugin"}
				plugins := NewRabbitMQPlugins(morePlugins)

				Expect(plugins.DesiredPlugins()).To(ConsistOf([]string{"rabbitmq_peer_discovery_k8s",
					"rabbitmq_prometheus",
					"rabbitmq_management",
					"my_great_plugin",
					"rabbitmq_shovel",
				}))
			})
		})

		When("AdditionalPlugins are provided with duplicates", func() {
			It("returns a unique list of plugins", func() {
				morePlugins := []rabbitmqv1beta1.Plugin{"rabbitmq_management", "rabbitmq_shovel", "my_great_plugin", "rabbitmq_shovel"}
				plugins := NewRabbitMQPlugins(morePlugins)

				Expect(plugins.DesiredPlugins()).To(ConsistOf([]string{"rabbitmq_peer_discovery_k8s",
					"rabbitmq_prometheus",
					"rabbitmq_management",
					"my_great_plugin",
					"rabbitmq_shovel",
				}))
			})
		})
	})
})
