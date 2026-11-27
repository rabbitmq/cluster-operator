/*
RabbitMQ Cluster Operator

Copyright 2026 Broadcom, Inc. All Rights Reserved.

This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the License.
*/

package controllers

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/v2/internal/resource"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestAnnotateIfNeededCreatedAt(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	scheme := runtime.NewScheme()
	g.Expect(appsv1.AddToScheme(scheme)).To(Succeed())
	g.Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())

	rmq := &rabbitmqv1beta1.RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-resize", Namespace: "default"},
	}
	stsName := rmq.ChildResourceName("server")
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: stsName, Namespace: rmq.Namespace},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts).Build()
	r := &RabbitmqClusterReconciler{
		Client:    c,
		APIReader: c,
		Scheme:    scheme,
		Recorder:  record.NewFakeRecorder(10),
	}
	builder := &resource.StatefulSetBuilder{RabbitmqResourceBuilder: &resource.RabbitmqResourceBuilder{Instance: rmq, Scheme: scheme}}

	// A PVC resize deletes and recreates the STS; CreateOrUpdate reports Created even
	// though the cluster is not new. The caller knows a StatefulSet existed before.
	g.Expect(r.annotateIfNeeded(ctx, log.Log, builder, controllerutil.OperationResultCreated, rmq, true)).To(Succeed())

	updated := &appsv1.StatefulSet{}
	g.Expect(c.Get(ctx, client.ObjectKeyFromObject(sts), updated)).To(Succeed())
	g.Expect(updated.Annotations).NotTo(HaveKey(stsCreateAnnotation))
}

func TestAnnotateIfNeededMarksNewStatefulSet(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	scheme := runtime.NewScheme()
	g.Expect(appsv1.AddToScheme(scheme)).To(Succeed())
	g.Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())

	rmq := &rabbitmqv1beta1.RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "new-cluster", Namespace: "default"},
	}
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: rmq.ChildResourceName("server"), Namespace: rmq.Namespace},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sts).Build()
	r := &RabbitmqClusterReconciler{Client: c, APIReader: c, Scheme: scheme, Recorder: record.NewFakeRecorder(10)}
	builder := &resource.StatefulSetBuilder{RabbitmqResourceBuilder: &resource.RabbitmqResourceBuilder{Instance: rmq, Scheme: scheme}}

	g.Expect(r.annotateIfNeeded(ctx, log.Log, builder, controllerutil.OperationResultCreated, rmq, false)).To(Succeed())
	updated := &appsv1.StatefulSet{}
	g.Expect(c.Get(ctx, client.ObjectKeyFromObject(sts), updated)).To(Succeed())
	g.Expect(updated.Annotations).To(HaveKey(stsCreateAnnotation))
}
