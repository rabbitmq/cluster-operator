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
	"k8s.io/utils/pointer"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"testing"

	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo/v2"
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
	rmqClusterClient client.Client
	clientSet        *kubernetes.Clientset
	namespace        string
	storageClassName = "persistent-test"
)

var _ = BeforeSuite(func() {
	scheme := runtime.NewScheme()
	Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
	Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())

	restConfig := controllerruntime.GetConfigOrDie()

	var err error
	rmqClusterClient, err = client.New(restConfig, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	clientSet, err = createClientSet()
	Expect(err).NotTo(HaveOccurred())

	namespace = MustHaveEnv("NAMESPACE")

	// Create or update the StorageClass used in persistence expansion test spec
	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: storageClassName,
		},
		Provisioner:          "kubernetes.io/gce-pd",
		AllowVolumeExpansion: pointer.BoolPtr(true),
	}
	ctx := context.Background()
	err = rmqClusterClient.Create(ctx, storageClass)
	if apierrors.IsAlreadyExists(err) {
		Expect(rmqClusterClient.Update(ctx, storageClass)).To(Succeed())
	} else {
		Expect(err).NotTo(HaveOccurred())
	}

	Eventually(func() int32 {
		operatorDeployment, err := clientSet.AppsV1().Deployments(namespace).Get(ctx, "rabbitmq-cluster-operator", metav1.GetOptions{})
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		return operatorDeployment.Status.ReadyReplicas
	}, 10, 1).Should(BeNumerically("==", 1), "Expected to have Operator Pod Ready")
})

var _ = AfterSuite(func() {
	_ = clientSet.StorageV1().StorageClasses().Delete(context.TODO(), storageClassName, metav1.DeleteOptions{})
})
