/*
RabbitMQ Cluster Operator

Copyright 2020 VMware, Inc. All Rights Reserved.

This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package controllers_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"k8s.io/client-go/util/retry"

	rabbithole "github.com/michaelklishin/rabbit-hole/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	controllers "github.com/rabbitmq/cluster-operator/v2/internal/controller"
	webhookv1beta1 "github.com/rabbitmq/cluster-operator/v2/internal/webhook/v1beta1"
	"github.com/rabbitmq/cluster-operator/v2/internal/rabbitmqclient"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

const (
	controllerName          = "rabbitmqcluster-controller"
	defaultRabbitmqImage    = "default-rabbit-image:stable"
	defaultUserUpdaterImage = "default-UU-image:unstable"
	defaultImagePullSecrets = "image-secret-1,image-secret-2,image-secret-3"
)

var (
	testEnv             *envtest.Environment
	client              runtimeClient.Client
	clientSet           *kubernetes.Clientset
	fakeExecutor        *fakePodExecutor
	fakeRabbitmqFactory *fakeRabbitmqClientFactory
	ctx                 context.Context
	cancel              context.CancelFunc
	updateWithRetry     = func(cr *rabbitmqv1beta1.RabbitmqCluster, mutateFn func(r *rabbitmqv1beta1.RabbitmqCluster)) error {
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
	RunSpecs(t, "Controller Suite", Label("integration"))
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))

	ctx, cancel = context.WithCancel(ctrl.SetupSignalHandler())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")},
		Config: &rest.Config{
			Host: fmt.Sprintf("http://localhost:218%d", GinkgoParallelProcess()),
		},
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "config", "webhook")},
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	Expect(scheme.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(rabbitmqv1beta1.AddToScheme(scheme.Scheme)).To(Succeed())

	clientSet, err = kubernetes.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())

	webhookInstallOptions := &testEnv.WebhookInstallOptions
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Metrics: server.Options{
			BindAddress: "0",
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    webhookInstallOptions.LocalServingHost,
			Port:    webhookInstallOptions.LocalServingPort,
			CertDir: webhookInstallOptions.LocalServingCertDir,
		}),
	})
	Expect(err).ToNot(HaveOccurred())

	err = webhookv1beta1.SetupRabbitmqClusterWebhookWithManager(mgr, webhookv1beta1.RabbitmqClusterCustomDefaulter{
		DefaultRabbitmqImage:    defaultRabbitmqImage,
		DefaultImagePullSecrets: defaultImagePullSecrets,
		DefaultUserUpdaterImage: defaultUserUpdaterImage,
	})
	Expect(err).ToNot(HaveOccurred())

	fakeExecutor = &fakePodExecutor{}
	fakeRabbitmqFactory = &fakeRabbitmqClientFactory{}

	err = (&controllers.RabbitmqClusterReconciler{
		Client:                  mgr.GetClient(),
		APIReader:               mgr.GetAPIReader(),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controllerName),
		Namespace:               "rabbitmq-system",
		Clientset:               clientSet,
		PodExecutor:             fakeExecutor,
		RabbitmqClientFactory:   fakeRabbitmqFactory,
		DefaultRabbitmqImage:    defaultRabbitmqImage,
		ControlRabbitmqImage:    false,
		DefaultUserUpdaterImage: defaultUserUpdaterImage,
		DefaultImagePullSecrets: defaultImagePullSecrets,
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = mgr.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

	client = mgr.GetClient()
	Expect(client).ToNot(BeNil())
})

var _ = AfterSuite(func() {
	cancel()
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

type fakeRabbitmqClientFactory struct {
	client *fakeRabbitmqClient
}

func (f *fakeRabbitmqClientFactory) GetClientForPod(ctx context.Context, k8sClient runtimeClient.Reader, rmq *rabbitmqv1beta1.RabbitmqCluster, podName string) (rabbitmqclient.RabbitmqClient, error) {
	if f.client == nil {
		return &fakeRabbitmqClient{}, nil
	}
	return f.client, nil
}

func (f *fakeRabbitmqClientFactory) GetClientForService(ctx context.Context, k8sClient runtimeClient.Reader, rmq *rabbitmqv1beta1.RabbitmqCluster) (rabbitmqclient.RabbitmqClient, error) {
	if f.client == nil {
		return &fakeRabbitmqClient{}, nil
	}
	return f.client, nil
}

type fakeRabbitmqClient struct {
	overview           *rabbithole.Overview
	deprecatedFeatures []rabbithole.DeprecatedFeature
	err                error
}

func (f *fakeRabbitmqClient) Overview() (*rabbithole.Overview, error) {
	if f.overview == nil && f.err == nil {
		return &rabbithole.Overview{
			RabbitMQVersion: "3.13.0",
			ErlangVersion:   "26.2.1",
		}, nil
	}
	return f.overview, f.err
}

func (f *fakeRabbitmqClient) ListDeprecatedFeaturesUsed() ([]rabbithole.DeprecatedFeature, error) {
	if f.deprecatedFeatures == nil && f.err == nil {
		return []rabbithole.DeprecatedFeature{}, nil
	}
	return f.deprecatedFeatures, f.err
}

func (f *fakeRabbitmqClient) HealthCheckNodeIsQuorumCritical() (rabbithole.HealthCheckStatus, error) {
	// Not used in reconcile_cli_test, mock realistically
	res := rabbithole.HealthCheckStatus{Status: "ok"}
	return res, nil
}

var _ = AfterEach(func() {
	fakeExecutor.ResetExecutedCommands()
	fakeRabbitmqFactory.client = nil
})
