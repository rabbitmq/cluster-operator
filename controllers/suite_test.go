/*
RabbitMQ Cluster Operator

Copyright 2020 VMware, Inc. All Rights Reserved.

This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package controllers_test

import (
	"context"
	"path/filepath"
	"testing"

	"k8s.io/client-go/util/retry"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/controllers"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

const controllerName = "rabbitmqcluster-controller"

var (
	testEnv         *envtest.Environment
	client          runtimeClient.Client
	clientSet       *kubernetes.Clientset
	fakeExecutor    *fakePodExecutor
	ctx             = context.Background()
	updateWithRetry = func(cr *rabbitmqv1beta1.RabbitmqCluster, mutateFn func(r *rabbitmqv1beta1.RabbitmqCluster)) error {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := client.Get(ctx, runtimeClient.ObjectKeyFromObject(cr), cr); err != nil {
				return err
			}
			mutateFn(cr)
			return client.Update(ctx, cr)
		})
	}
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	Expect(scheme.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(rabbitmqv1beta1.AddToScheme(scheme.Scheme)).To(Succeed())

	clientSet, err = kubernetes.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	fakeExecutor = &fakePodExecutor{}
	err = (&controllers.RabbitmqClusterReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		Recorder:    mgr.GetEventRecorderFor(controllerName),
		Namespace:   "rabbitmq-system",
		PodExecutor: fakeExecutor,
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = mgr.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	client = mgr.GetClient()
	Expect(client).ToNot(BeNil())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Expect(testEnv.Stop()).To(Succeed())
})

type fakePodExecutor struct {
	executedCommands []command
}

type command []string

func (f *fakePodExecutor) Exec(clientset *kubernetes.Clientset, clusterConfig *rest.Config, namespace, podName, containerName string, command ...string) (string, string, error) {
	f.executedCommands = append(f.executedCommands, command)
	return "", "", nil
}

func (f *fakePodExecutor) ExecutedCommands() []command { return f.executedCommands }

func (f *fakePodExecutor) ResetExecutedCommands() { f.executedCommands = []command{} }

var _ = AfterEach(func() { fakeExecutor.ResetExecutedCommands() })
