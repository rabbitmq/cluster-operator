/*
Copyright 2019 Pivotal.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers_test

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/controllers"

	// "github.com/pivotal/rabbitmq-for-kubernetes/internal/config"

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
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	os.Setenv("IGNORE_POD_EXECUTE", "true")

	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	clientSet, err = kubernetes.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())

	scheme = runtime.NewScheme()
	Expect(rabbitmqv1beta1.AddToScheme(scheme)).NotTo(HaveOccurred())
	Expect(defaultscheme.AddToScheme(scheme)).NotTo(HaveOccurred())

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
		Exec:      fakeExec,
		Client:    client,
		Log:       ctrl.Log.WithName(controllerName),
		Scheme:    mgr.GetScheme(),
		Recorder:  mgr.GetEventRecorderFor(controllerName),
		Namespace: "pivotal-rabbitmq-system",
	}
	Expect(reconciler.SetupWithManager(mgr)).To(Succeed())

	stopMgr = make(chan struct{})
	mgrStopped = &sync.WaitGroup{}
	mgrStopped.Add(1)
	go func() {
		defer mgrStopped.Done()
		Expect(mgr.Start(stopMgr)).NotTo(HaveOccurred())
	}()
}

func fakeExec(namespace, podName, containerName string, command ...string) (string, error) {
	return "", errors.New("fake error")
}
