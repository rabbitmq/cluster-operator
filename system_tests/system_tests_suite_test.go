// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
//

package system_tests

import (
	"context"
	"testing"

	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSystemTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SystemTests Suite")
}

var (
	rmqClusterClient          client.Client
	clientSet                 *kubernetes.Clientset
	namespace                 string
	specifiedStorageClassName = "persistent-test"
)

var _ = BeforeSuite(func() {
	scheme := runtime.NewScheme()
	Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
	Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())

	restConfig, err := createRestConfig()
	Expect(err).NotTo(HaveOccurred())

	rmqClusterClient, err = client.New(restConfig, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())

	clientSet, err = createClientSet()
	Expect(err).NotTo(HaveOccurred())

	namespace = MustHaveEnv("NAMESPACE")

	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: specifiedStorageClassName,
		},
		Provisioner: "kubernetes.io/gce-pd",
	}
	err = rmqClusterClient.Create(context.TODO(), storageClass)
	if !apierrors.IsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred())
	}

	Eventually(func() []byte {
		output, err := kubectl(
			"-n",
			namespace,
			"get",
			"deployment",
			"-l",
			"app.kubernetes.io/name=rabbitmq-cluster-operator",
		)

		Expect(err).NotTo(HaveOccurred())

		return output
	}, 10, 1).Should(ContainSubstring("1/1"))
})
