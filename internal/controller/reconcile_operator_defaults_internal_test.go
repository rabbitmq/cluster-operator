/*
RabbitMQ Cluster Operator

Copyright 2026 Broadcom, Inc. All Rights Reserved.

This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.

This product may include a number of subcomponents with separate copyright notices and License terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
				Name:      "test-no-update",
				Namespace: "default",
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
				Name:      "test-control-image",
				Namespace: "default",
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

var _ = Describe("image enforcement patches", func() {
	It("patches only images once and skips subsequent writes", func(ctx SpecContext) {
		scheme := runtime.NewScheme()
		Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
		cluster := &rabbitmqv1beta1.RabbitmqCluster{Name: "images", Namespace: "default"}
		cluster.Spec.SecretBackend.Vault = &rabbitmqv1beta1.VaultSpec{DefaultUserPath: "path"}
		calls := 0
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).
			WithInterceptorFuncs(interceptor.Funcs{Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				calls++
				data, err := patch.Data(obj)
				Expect(err).NotTo(HaveOccurred())
				var payload map[string]interface{}
				Expect(json.Unmarshal(data, &payload)).To(Succeed())
				Expect(payload).To(HaveLen(2))
				Expect(payload).To(HaveKey("metadata"))
				Expect(payload["spec"]).To(Equal(map[string]interface{}{
					"image":         "rabbit:new",
					"secretBackend": map[string]interface{}{"vault": map[string]interface{}{"defaultUserUpdaterImage": "helper:new"}},
				}))
				return c.Patch(ctx, obj, patch, opts...)
			}}).Build()
		r := &RabbitmqClusterReconciler{Client: c, ControlRabbitmqImage: true, DefaultRabbitmqImage: "rabbit:new", DefaultUserUpdaterImage: "helper:new"}
		delay, err := r.reconcileOperatorDefaults(ctx, cluster)
		Expect(err).NotTo(HaveOccurred())
		Expect(delay).To(BeZero())
		_, err = r.reconcileOperatorDefaults(ctx, cluster)
		Expect(err).NotTo(HaveOccurred())
		Expect(calls).To(Equal(1))
	})

	DescribeTable("returns patch failures without continuing",
		func(ctx SpecContext, failure error, delay time.Duration) {
			scheme := runtime.NewScheme()
			Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
			cluster := &rabbitmqv1beta1.RabbitmqCluster{Name: "failure", Namespace: "default"}
			calls := 0
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).
				WithInterceptorFuncs(interceptor.Funcs{Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
					calls++
					return failure
				}}).Build()
			r := &RabbitmqClusterReconciler{Client: c, ControlRabbitmqImage: true, DefaultRabbitmqImage: "new"}
			got, err := r.reconcileOperatorDefaults(ctx, cluster)
			Expect(got).To(Equal(delay))
			if delay > 0 {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(MatchError(failure))
			}
			Expect(calls).To(Equal(1))
			Expect(cluster.Spec.SecretBackend.Vault).To(BeNil())
		},
		Entry("conflict", k8serrors.NewConflict(schema.GroupResource{Group: "rabbitmq.com", Resource: "rabbitmqclusters"}, "failure", errors.New("stale")), 2*time.Second),
		Entry("other error", errors.New("unavailable"), time.Duration(0)),
	)
})

var _ = Describe("metadata patch retries", func() {
	It("preserves concurrent annotations after a conflict", func(ctx SpecContext) {
		scheme := runtime.NewScheme()
		Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
		cluster := &rabbitmqv1beta1.RabbitmqCluster{Name: "annotations", Namespace: "default"}
		calls := 0
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).
			WithInterceptorFuncs(interceptor.Funcs{Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				calls++
				if calls == 1 {
					current := &rabbitmqv1beta1.RabbitmqCluster{}
					Expect(c.Get(ctx, client.ObjectKeyFromObject(obj), current)).To(Succeed())
					current.Annotations = map[string]string{"other": "preserved"}
					Expect(c.Update(ctx, current)).To(Succeed())
				}
				return c.Patch(ctx, obj, patch, opts...)
			}}).Build()
		r := &RabbitmqClusterReconciler{Client: c}
		Expect(r.updateAnnotation(ctx, cluster, cluster.Namespace, cluster.Name, "operator", "value")).To(Succeed())
		Expect(calls).To(Equal(2))
		Expect(cluster.Annotations).To(Equal(map[string]string{"other": "preserved", "operator": "value"}))
		Expect(r.updateAnnotation(ctx, cluster, cluster.Namespace, cluster.Name, "operator", "value")).To(Succeed())
		Expect(r.deleteAnnotation(ctx, cluster, "absent")).To(Succeed())
		Expect(calls).To(Equal(2))
	})
})

var _ = Describe("finalizer refresh", func() {
	It("does not retain deleted metadata or mutate the caller on a failed write", func(ctx SpecContext) {
		scheme := runtime.NewScheme()
		Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
		stored := &rabbitmqv1beta1.RabbitmqCluster{Name: "finalizers", Namespace: "default"}
		caller := stored.DeepCopy()
		caller.Annotations = map[string]string{"removed": "stale"}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(stored).Build()
		r := &RabbitmqClusterReconciler{Client: c}
		Expect(r.addFinalizerIfNeeded(ctx, caller)).To(Succeed())
		Expect(caller.Annotations).To(BeEmpty())
		Expect(caller.Finalizers).To(ContainElement(deletionFinalizer))
		Expect(r.removeFinalizer(ctx, caller)).To(Succeed())
		Expect(caller.Finalizers).To(BeEmpty())

		failure := errors.New("write failed")
		failing := fake.NewClientBuilder().WithScheme(scheme).WithObjects(stored).
			WithInterceptorFuncs(interceptor.Funcs{Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
				return failure
			}}).Build()
		r.Client = failing
		before := caller.DeepCopy()
		Expect(r.addFinalizerIfNeeded(ctx, caller)).To(MatchError(failure))
		Expect(caller).To(Equal(before))
	})
})
