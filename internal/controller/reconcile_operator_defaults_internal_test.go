/*
RabbitMQ Cluster Operator

Copyright 2026 Broadcom, Inc. All Rights Reserved.

This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.

This product may include a number of subcomponents with separate copyright notices and License terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package controllers

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var _ = Describe("reconcileOperatorDefaults", func() {
	When("ControlRabbitmqImage is false", func() {
		It("does not call Update", func(ctx SpecContext) {

			scheme := runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())

			cluster := &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-no-update",
					Namespace: "default",
				},
			}

			updateCallCount := 0
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(cluster).
				WithInterceptorFuncs(interceptor.Funcs{
					Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
						updateCallCount++
						return c.Update(ctx, obj, opts...)
					},
				}).
				Build()

			reconciler := &RabbitmqClusterReconciler{
				Client:                  fakeClient,
				Scheme:                  scheme,
				DefaultRabbitmqImage:    "default-rabbit-image:stable",
				DefaultUserUpdaterImage: "default-uu-image:unstable",
				DefaultImagePullSecrets: "secret-1,secret-2",
				ControlRabbitmqImage:    false,
			}

			requeue, err := reconciler.reconcileOperatorDefaults(ctx, cluster)

			Expect(err).NotTo(HaveOccurred())
			Expect(requeue).To(BeZero())
			Expect(updateCallCount).To(Equal(0))
		})
	})

	When("ControlRabbitmqImage is true", func() {
		It("enforces the default image and user updater image", func(ctx SpecContext) {

			scheme := runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())

			cluster := &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-control-image",
					Namespace: "default",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					Image: "user-set-image:1.0",
					SecretBackend: rabbitmqv1beta1.SecretBackend{
						Vault: &rabbitmqv1beta1.VaultSpec{
							DefaultUserPath: "some-path",
						},
					},
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(cluster).
				Build()

			reconciler := &RabbitmqClusterReconciler{
				Client:                  fakeClient,
				Scheme:                  scheme,
				DefaultRabbitmqImage:    "controlled-image:latest",
				DefaultUserUpdaterImage: "controlled-uu-image:latest",
				ControlRabbitmqImage:    true,
			}

			requeue, err := reconciler.reconcileOperatorDefaults(ctx, cluster)

			Expect(err).NotTo(HaveOccurred())
			Expect(requeue).To(BeZero())
			Expect(cluster.Spec.Image).To(Equal("controlled-image:latest"))
			Expect(cluster.Spec.SecretBackend.Vault.DefaultUserUpdaterImage).To(PointTo(Equal("controlled-uu-image:latest")))
		})
	})
})
