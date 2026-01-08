/*
RabbitMQ Cluster Operator

Copyright 2020 VMware, Inc. All Rights Reserved.

This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package rabbitmqclient

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
)

var _ = Describe("RabbitMQ Client", func() {
	var (
		ctx       context.Context
		k8sClient client.Client
		scheme    *runtime.Scheme
		rmq       *rabbitmqv1beta1.RabbitmqCluster
		secret    *corev1.Secret
		namespace string
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespace = "test-namespace"

		scheme = runtime.NewScheme()
		Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		rmq = &rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: namespace,
			},
			Spec: rabbitmqv1beta1.RabbitmqClusterSpec{},
		}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster-default-user",
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"username": []byte("test-user"),
				"password": []byte("test-password"),
			},
		}
	})

	Describe("GetClientInfoForPod", func() {
		Context("when the secret exists", func() {
			BeforeEach(func() {
				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(rmq, secret).
					Build()
			})

			It("returns client info with correct credentials", func() {
				podIP := "10.0.0.1"
				info, err := GetClientInfoForPod(ctx, k8sClient, rmq, podIP)
				Expect(err).NotTo(HaveOccurred())
				Expect(info).NotTo(BeNil())
				Expect(info.Username).To(Equal("test-user"))
				Expect(info.Password).To(Equal("test-password"))
			})

			It("returns the correct base URL for non-TLS with pod IP", func() {
				podIP := "10.0.0.1"
				info, err := GetClientInfoForPod(ctx, k8sClient, rmq, podIP)
				Expect(err).NotTo(HaveOccurred())
				Expect(info.BaseURL).To(Equal("http://10.0.0.1:15672"))
			})

			It("returns the correct base URL for TLS with pod IP", func() {
				rmq.Spec.TLS.SecretName = "tls-secret"
				podIP := "10.0.0.2"
				info, err := GetClientInfoForPod(ctx, k8sClient, rmq, podIP)
				Expect(err).NotTo(HaveOccurred())
				Expect(info.BaseURL).To(Equal("https://10.0.0.2:15671"))
			})

			It("returns an HTTP transport for TLS", func() {
				rmq.Spec.TLS.SecretName = "tls-secret"
				podIP := "10.0.0.1"
				info, err := GetClientInfoForPod(ctx, k8sClient, rmq, podIP)
				Expect(err).NotTo(HaveOccurred())
				Expect(info.Transport).NotTo(BeNil())
				Expect(info.Transport.TLSClientConfig).NotTo(BeNil())
			})

			It("returns nil transport for non-TLS", func() {
				podIP := "10.0.0.1"
				info, err := GetClientInfoForPod(ctx, k8sClient, rmq, podIP)
				Expect(err).NotTo(HaveOccurred())
				Expect(info.Transport).To(BeNil())
			})
		})

		Context("when the secret does not exist", func() {
			BeforeEach(func() {
				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(rmq).
					Build()
			})

			It("returns an error", func() {
				podIP := "10.0.0.1"
				_, err := GetClientInfoForPod(ctx, k8sClient, rmq, podIP)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get default user secret"))
			})
		})
	})

	Describe("GetRabbitmqClientForPod", func() {
		Context("when TLS is disabled", func() {
			It("returns a non-TLS client", func() {
				podIP := "10.0.0.1"
				client, err := GetRabbitmqClientForPod(ctx, k8sClient, rmq, podIP)
				Expect(err).NotTo(HaveOccurred())
				Expect(client).NotTo(BeNil())
			})
		})

		Context("when TLS is enabled", func() {
			It("returns a TLS client", func() {
				rmq.Spec.TLS.SecretName = "tls-secret"
				podIP := "10.0.0.1"
				client, err := GetRabbitmqClientForPod(ctx, k8sClient, rmq, podIP)
				Expect(err).NotTo(HaveOccurred())
				Expect(client).NotTo(BeNil())
			})
		})

		Context("when the secret does not exist", func() {
			BeforeEach(func() {
				k8sClient = fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(rmq).
					Build()
			})

			It("returns an error", func() {
				podIP := "10.0.0.1"
				_, err := GetRabbitmqClientForPod(ctx, k8sClient, rmq, podIP)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get default user secret"))
			})
		})
	})
})
