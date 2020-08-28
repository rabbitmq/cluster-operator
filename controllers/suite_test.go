/*
RabbitMQ Cluster Operator

Copyright 2020 VMware, Inc. All Rights Reserved.

This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package controllers_test

import (
	"path/filepath"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/controllers"

	// "github.com/rabbitmq/cluster-operator/internal/config"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
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
	cfg        *rest.Config
	testEnv    *envtest.Environment
	client     runtimeClient.Client
	clientSet  *kubernetes.Clientset
	stopMgr    chan struct{}
	mgrStopped *sync.WaitGroup
	scheme     *runtime.Scheme
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	var err error
	logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	clientSet, err = kubernetes.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())

	scheme = runtime.NewScheme()
	Expect(rabbitmqv1beta1.AddToScheme(scheme)).To(Succeed())
	Expect(defaultscheme.AddToScheme(scheme)).To(Succeed())

	startManager(scheme)
})

var _ = AfterSuite(func() {
	close(stopMgr)
	mgrStopped.Wait()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func startManager(scheme *runtime.Scheme) {
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	client = mgr.GetClient()

	reconciler := &controllers.RabbitmqClusterReconciler{
		Client:    client,
		Log:       ctrl.Log.WithName(controllerName),
		Scheme:    mgr.GetScheme(),
		Recorder:  mgr.GetEventRecorderFor(controllerName),
		Namespace: "rabbitmq-system",
	}
	Expect(reconciler.SetupWithManager(mgr)).To(Succeed())

	stopMgr = make(chan struct{})
	mgrStopped = &sync.WaitGroup{}
	mgrStopped.Add(1)
	go func() {
		defer mgrStopped.Done()
		Expect(mgr.Start(stopMgr)).To(Succeed())
	}()
}
