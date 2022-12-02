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
	"embed"
	"fmt"
	"github.com/google/uuid"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"testing"

	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
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
	rabbitClient     *rabbithole.Client
	clientSet        *kubernetes.Clientset
	scheme           = runtime.NewScheme()
	restConfig       = controllerruntime.GetConfigOrDie()
	rmq              *rabbitmqv1beta1.RabbitmqCluster
	tlsSecretName    = fmt.Sprintf("rmq-test-cert-%v", uuid.New())
	namespace        = MustHaveEnv("NAMESPACE")
	//go:embed fixtures
	fixtures embed.FS
)

var _ = SynchronizedBeforeSuite(func() {
	Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
	Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())

	var err error
	rmqClusterClient, err = client.New(restConfig, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	clientSet, err = createClientSet()
	Expect(err).NotTo(HaveOccurred())

	Eventually(func() int32 {
		operatorDeployment, err := clientSet.AppsV1().Deployments(namespace).Get(context.Background(), "rabbitmq-cluster-operator", metav1.GetOptions{})
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		return operatorDeployment.Status.ReadyReplicas
	}, 10, 1).Should(BeNumerically("==", 1), "Expected to have Operator Pod Ready")

	_, _, _ = createTLSSecret(tlsSecretName, namespace, "tls-cluster.rabbitmq-system.svc")

	patchBytes, _ := fixtures.ReadFile("fixtures/patch-test-ca.yaml")
	_, err = kubectl(
		"-n",
		namespace,
		"patch",
		"deployment",
		"rabbitmq-cluster-operator",
		"--patch",
		fmt.Sprintf(string(patchBytes), tlsSecretName+"-ca"),
	)
	Expect(err).NotTo(HaveOccurred())
}, func() {
	Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
	Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())

	var err error
	rmqClusterClient, err = client.New(restConfig, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	clientSet, err = createClientSet()
	Expect(err).NotTo(HaveOccurred())

	// setup a RabbitmqCluster used for system tests
	rmq = basicTestRabbitmqCluster(fmt.Sprintf("system-test-node-%d", GinkgoParallelProcess()), namespace)
	setupTestRabbitmqCluster(rmqClusterClient, rmq)

	rabbitClient, err = generateRabbitClient(context.Background(), clientSet, namespace, rmq.Name)
	Expect(err).NotTo(HaveOccurred())
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	Expect(rmqClusterClient.Delete(context.Background(), &rabbitmqv1beta1.RabbitmqCluster{ObjectMeta: metav1.ObjectMeta{Name: rmq.Name, Namespace: rmq.Namespace}})).ToNot(HaveOccurred())
})
