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
	"k8s.io/utils/ptr"

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

	Context("override validation", func() {
		var validator RabbitmqClusterCustomValidator

		BeforeEach(func() {
			validator = RabbitmqClusterCustomValidator{}
		})

		It("allows a cluster with no override", func() {
			_, err := validator.ValidateCreate(context.Background(), obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("allows a cluster with a safe pod spec override", func() {
			obj.Spec.Override.StatefulSet = &rabbitmqcomv1beta1.StatefulSet{
				Spec: &rabbitmqcomv1beta1.StatefulSetSpec{
					Template: &rabbitmqcomv1beta1.PodTemplateSpec{
						Spec: &corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "rabbitmq", Image: "rabbitmq:custom"},
							},
						},
					},
				},
			}
			_, err := validator.ValidateCreate(context.Background(), obj)
			Expect(err).NotTo(HaveOccurred())
		})

		DescribeTableSubtree("rejecting forbidden fields",
			func(podSpec *corev1.PodSpec, expectedFragment string) {
				BeforeEach(func() {
					obj.Spec.Override.StatefulSet = &rabbitmqcomv1beta1.StatefulSet{
						Spec: &rabbitmqcomv1beta1.StatefulSetSpec{
							Template: &rabbitmqcomv1beta1.PodTemplateSpec{Spec: podSpec},
						},
					}
				})

				It("rejects forbidden fields in create", func() {
					_, err := validator.ValidateCreate(context.Background(), obj)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(expectedFragment))
				})

				It("rejects forbidden fields in update", func() {
					_, err := validator.ValidateUpdate(context.Background(), obj, obj)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(expectedFragment))
				})
			},
			Entry("hostPID", &corev1.PodSpec{HostPID: true}, "hostPID"),
			Entry("hostNetwork", &corev1.PodSpec{HostNetwork: true}, "hostNetwork"),
			Entry("hostIPC", &corev1.PodSpec{HostIPC: true}, "hostIPC"),
			Entry("serviceAccountName", &corev1.PodSpec{ServiceAccountName: "other-sa"}, "serviceAccountName"),
			Entry("hostPath volume", &corev1.PodSpec{
				Volumes: []corev1.Volume{
					{Name: "host-vol", VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{Path: "/"},
					}},
				},
			}, "hostPath"),
			Entry("privileged container", &corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "evil", SecurityContext: &corev1.SecurityContext{Privileged: ptr.To(true)}},
				},
			}, "privileged"),
			Entry("privileged initContainer", &corev1.PodSpec{
				InitContainers: []corev1.Container{
					{Name: "evil-init", SecurityContext: &corev1.SecurityContext{Privileged: ptr.To(true)}},
				},
			}, "privileged"),
			Entry("allowPrivilegeEscalation container", &corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "esc", SecurityContext: &corev1.SecurityContext{AllowPrivilegeEscalation: ptr.To(true)}},
				},
			}, "allowPrivilegeEscalation"),
		)
	})
})
