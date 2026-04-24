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
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var _ = Describe("reconcileOperatorDefaults", func() {
	When("DEFAULT_IMAGE_PULL_SECRETS is empty", func() {
		It("does not Update the cluster when ImagePullSecrets is nil", func() {
			ctx := context.Background()

			scheme := runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())

			presetUserUpdaterImage := "preset-uu-image:1.0"
			cluster := &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-no-defaults",
					Namespace: "default",
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					// Pre-set Image and DefaultUserUpdaterImage so the only branch
					// in reconcileOperatorDefaults that could trigger an Update is
					// the ImagePullSecrets one we are testing.
					Image: "preset-image:1.0",
					SecretBackend: rabbitmqv1beta1.SecretBackend{
						Vault: &rabbitmqv1beta1.VaultSpec{
							DefaultUserPath:         "some-path",
							DefaultUserUpdaterImage: &presetUserUpdaterImage,
						},
					},
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
				DefaultImagePullSecrets: "",
			}

			By("invoking reconcileOperatorDefaults with empty DefaultImagePullSecrets")
			requeue, err := reconciler.reconcileOperatorDefaults(ctx, cluster)

			Expect(err).NotTo(HaveOccurred())
			Expect(requeue).To(BeZero())

			By("not calling Update on the cluster")
			Expect(updateCallCount).To(Equal(0))

			By("leaving in-memory ImagePullSecrets as nil")
			Expect(cluster.Spec.ImagePullSecrets).To(BeNil())

			By("leaving the persisted ImagePullSecrets as nil")
			fetched := &rabbitmqv1beta1.RabbitmqCluster{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(cluster), fetched)).To(Succeed())
			Expect(fetched.Spec.ImagePullSecrets).To(BeNil())
		})
	})
})
