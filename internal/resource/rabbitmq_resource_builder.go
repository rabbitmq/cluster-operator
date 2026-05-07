// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
//

package resource

import (
	"slices"

	"github.com/Masterminds/semver/v3"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RabbitmqResourceBuilder struct {
	Instance *rabbitmqv1beta1.RabbitmqCluster
	Scheme   *runtime.Scheme
}

type ResourceBuilder interface {
	Build() (client.Object, error)
	Update(client.Object) error
	UpdateMayRequireStsRecreate() bool
}

// peerDiscoveryRBACConstraint is the semver constraint used to determine whether
// peer-discovery RBAC (Role and RoleBinding) should be created. It is created
// once at package scope to avoid repeated allocations.
var peerDiscoveryRBACConstraint = mustNewConstraint(">= 4.1.0")

// mustNewConstraint creates a semver constraint and panics if parsing fails.
// This is only used for constant constraint strings that are always valid.
func mustNewConstraint(c string) *semver.Constraints {
	constraint, err := semver.NewConstraint(c)
	if err != nil {
		panic(err)
	}
	return constraint
}

// ShouldCreatePeerDiscoveryRBAC returns true if the peer-discovery Role and
// RoleBinding should be created for this RabbitmqCluster. The ServiceAccount is
// always created regardless of RabbitMQ version because other integrations (e.g.
// Vault Kubernetes auth) may rely on it.
func ShouldCreatePeerDiscoveryRBAC(rmq *rabbitmqv1beta1.RabbitmqCluster) bool {
	version := rmq.GetRabbitMQVersion()
	if version == rabbitmqv1beta1.VersionNotAnnotated {
		return true
	}

	v, err := semver.NewVersion(version)
	if err != nil {
		return true
	}

	return !peerDiscoveryRBACConstraint.Check(v)
}

func (builder *RabbitmqResourceBuilder) ResourceBuilders() []ResourceBuilder {

	builders := []ResourceBuilder{
		builder.HeadlessService(),
		builder.Service(),
		builder.ErlangCookie(),
		builder.DefaultUserSecret(),
		builder.RabbitmqPluginsConfigMap(),
		builder.ServerConfigMap(),
		builder.ServiceAccount(),
	}

	if ShouldCreatePeerDiscoveryRBAC(builder.Instance) {
		builders = append(builders,
			builder.Role(),
			builder.RoleBinding(),
		)
	}

	// Appending StatefulSet builder separately because the order of the builders is important
	// The SA, ConfigMap, and Secret need to be created before the StatefulSet. Otherwise, Pods
	// created by the StatefulSet will block on the creation of dependent resources.
	builders = append(builders, builder.StatefulSet())

	if builder.Instance.VaultDefaultUserSecretEnabled() || builder.Instance.ExternalSecretEnabled() {
		// do not create default-user K8s Secret
		builders = slices.Delete(builders, 3, 3+1)
	}
	return builders
}
