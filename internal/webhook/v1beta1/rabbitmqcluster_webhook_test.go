/*
RabbitMQ Cluster Operator

Copyright 2020-2022 VMware, Inc. All Rights Reserved.
Copyright 2022-2026 Broadcom. All Rights Reserved.

This product is licensed to you under the Mozilla Public license,
Version 2.0 (the "License").  You may not use this product except in
compliance with the Mozilla Public License.

This product may include a number of subcomponents with separate
copyright notices and license terms. Your use of these subcomponents
is subject to the terms and conditions of the subcomponent's license,
as noted in the LICENSE file.
*/

package v1beta1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rabbitmqcomv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
)

var _ = Describe("RabbitmqCluster Webhook", func() {
	var (
		obj       *rabbitmqcomv1beta1.RabbitmqCluster
		defaulter RabbitmqClusterCustomDefaulter
	)

	BeforeEach(func() {
		obj = &rabbitmqcomv1beta1.RabbitmqCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
		}
		defaulter = RabbitmqClusterCustomDefaulter{
			DefaultRabbitmqImage:    "rabbitmq:default",
			DefaultImagePullSecrets: "secret-1,secret-2",
			DefaultUserUpdaterImage: "credential-updater:default",
		}
	})

	Context("image defaulting", func() {
		It("sets the default image when spec.image is empty", func() {
			Expect(defaulter.Default(context.Background(), obj)).To(Succeed())
			Expect(obj.Spec.Image).To(Equal("rabbitmq:default"))
		})

		It("does not override an image the user has already set", func() {
			obj.Spec.Image = "rabbitmq:custom"
			Expect(defaulter.Default(context.Background(), obj)).To(Succeed())
			Expect(obj.Spec.Image).To(Equal("rabbitmq:custom"))
		})
	})

	Context("imagePullSecrets defaulting", func() {
		It("sets imagePullSecrets from the comma-separated default when spec.imagePullSecrets is nil", func() {
			Expect(defaulter.Default(context.Background(), obj)).To(Succeed())
			Expect(obj.Spec.ImagePullSecrets).To(ConsistOf(
				corev1.LocalObjectReference{Name: "secret-1"},
				corev1.LocalObjectReference{Name: "secret-2"},
			))
		})

		It("does not override imagePullSecrets the user has already set", func() {
			obj.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "user-secret"}}
			Expect(defaulter.Default(context.Background(), obj)).To(Succeed())
			Expect(obj.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "user-secret"}))
		})

		It("does not set imagePullSecrets when DefaultImagePullSecrets is empty", func() {
			defaulter.DefaultImagePullSecrets = ""
			Expect(defaulter.Default(context.Background(), obj)).To(Succeed())
			Expect(obj.Spec.ImagePullSecrets).To(BeNil())
		})

		It("ignores empty entries in the comma-separated list", func() {
			defaulter.DefaultImagePullSecrets = "secret-1,,secret-2"
			Expect(defaulter.Default(context.Background(), obj)).To(Succeed())
			Expect(obj.Spec.ImagePullSecrets).To(ConsistOf(
				corev1.LocalObjectReference{Name: "secret-1"},
				corev1.LocalObjectReference{Name: "secret-2"},
			))
		})
	})

	Context("default user updater image defaulting", func() {
		It("sets the default user updater image when vault is enabled and the field is nil", func() {
			obj.Spec.SecretBackend.Vault = &rabbitmqcomv1beta1.VaultSpec{DefaultUserPath: "some-path"}
			Expect(defaulter.Default(context.Background(), obj)).To(Succeed())
			Expect(obj.Spec.SecretBackend.Vault.DefaultUserUpdaterImage).To(PointTo(Equal("credential-updater:default")))
		})

		It("does not override a user updater image the user has already set", func() {
			custom := "credential-updater:custom"
			obj.Spec.SecretBackend.Vault = &rabbitmqcomv1beta1.VaultSpec{
				DefaultUserPath:         "some-path",
				DefaultUserUpdaterImage: &custom,
			}
			Expect(defaulter.Default(context.Background(), obj)).To(Succeed())
			Expect(obj.Spec.SecretBackend.Vault.DefaultUserUpdaterImage).To(PointTo(Equal("credential-updater:custom")))
		})

		It("does not set the user updater image when vault is not enabled", func() {
			Expect(defaulter.Default(context.Background(), obj)).To(Succeed())
			Expect(obj.Spec.SecretBackend.Vault).To(BeNil())
		})
	})
})
