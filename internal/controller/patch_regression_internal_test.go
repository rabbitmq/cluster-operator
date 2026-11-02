/*
RabbitMQ Cluster Operator

Copyright 2026 Broadcom, Inc. All Rights Reserved.

This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.

This product may include a number of subcomponents with separate copyright notices and License terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package controllers

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var _ = Describe("targeted CR patches", Label("integration"), func() {
	It("preserves numeric quantities and rejects stale Vault enforcement", func(ctx SpecContext) {
		// Isolate direct helper calls from the shared suite controller and webhook.
		env := &envtest.Environment{CRDDirectoryPaths: []string{"../../config/crd/bases"}}
		cfg, err := env.Start()
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(env.Stop()).To(Succeed()) })
		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
		c, err := client.New(cfg, client.Options{Scheme: scheme})
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "patches"}})).To(Succeed())
		desired := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "rabbitmq.com/v1beta1", "kind": "RabbitmqCluster",
			"metadata": map[string]interface{}{"name": "regression", "namespace": "patches"},
			"spec": map[string]interface{}{
				"image":         "rabbit:old",
				"resources":     map[string]interface{}{"limits": map[string]interface{}{"cpu": int64(1)}},
				"secretBackend": map[string]interface{}{"vault": map[string]interface{}{"defaultUserPath": "secret/user"}},
			},
		}}
		Expect(c.Patch(ctx, desired.DeepCopy(), client.Apply, client.FieldOwner("user"))).To(Succeed())
		key := client.ObjectKeyFromObject(desired)
		cluster := &rabbitmqv1beta1.RabbitmqCluster{}
		Expect(c.Get(ctx, key, cluster)).To(Succeed())
		r := &RabbitmqClusterReconciler{Client: c, APIReader: c, ControlRabbitmqImage: true, DefaultRabbitmqImage: "rabbit:new", DefaultUserUpdaterImage: "helper:new"}
		delay, err := r.reconcileOperatorDefaults(ctx, cluster)
		Expect(err).NotTo(HaveOccurred())
		Expect(delay).To(BeZero())
		Expect(r.addFinalizerIfNeeded(ctx, cluster)).To(Succeed())
		Expect(r.updateAnnotation(ctx, cluster, key.Namespace, key.Name, "test", "value")).To(Succeed())
		Expect(r.deleteAnnotation(ctx, cluster, "test")).To(Succeed())
		Expect(r.markForQueueRebalance(ctx, cluster)).To(Succeed())
		Expect(r.removeFinalizer(ctx, cluster)).To(Succeed())
		raw := desired.DeepCopy()
		Expect(c.Get(ctx, key, raw)).To(Succeed())
		cpu, found, err := unstructured.NestedInt64(raw.Object, "spec", "resources", "limits", "cpu")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(cpu).To(Equal(int64(1)))
		for _, entry := range raw.GetManagedFields() {
			if entry.Manager != "user" && entry.Subresource == "" && entry.FieldsV1 != nil {
				Expect(strings.Contains(string(entry.FieldsV1.Raw), "f:resources")).To(BeFalse())
			}
		}
		// The user acknowledges enforced images, then changes its own resource field.
		Expect(unstructured.SetNestedField(desired.Object, "rabbit:new", "spec", "image")).To(Succeed())
		Expect(unstructured.SetNestedField(desired.Object, int64(2), "spec", "resources", "limits", "cpu")).To(Succeed())
		Expect(c.Patch(ctx, desired.DeepCopy(), client.Apply, client.FieldOwner("user"))).To(Succeed())

		Expect(c.Get(ctx, key, cluster)).To(Succeed())
		stale := cluster.DeepCopy()
		base := cluster.DeepCopy()
		cluster.Spec.SecretBackend.Vault = nil
		Expect(c.Patch(ctx, cluster, client.MergeFrom(base))).To(Succeed())
		r.DefaultUserUpdaterImage = "helper:next"
		delay, err = r.reconcileOperatorDefaults(ctx, stale)
		Expect(err).NotTo(HaveOccurred())
		Expect(delay).NotTo(BeZero())
		Expect(c.Get(ctx, key, cluster)).To(Succeed())
		_, err = r.reconcileOperatorDefaults(ctx, cluster)
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Get(ctx, key, cluster)).To(Succeed())
		Expect(cluster.Spec.SecretBackend.Vault).To(BeNil())
	})
})
