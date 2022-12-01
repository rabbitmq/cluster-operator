/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package topologycontrollers_test

import (
	"context"
	"crypto/x509"
	"github.com/rabbitmq/cluster-operator/controllers/topologycontrollers"
	"path/filepath"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	"github.com/rabbitmq/cluster-operator/internal/rabbitmqclient"
	"github.com/rabbitmq/cluster-operator/internal/rabbitmqclient/rabbitmqclientfakes"

	rabbitmqv1alpha1 "github.com/rabbitmq/cluster-operator/api/v1alpha1"
	topologyClient "github.com/rabbitmq/cluster-operator/pkg/generated/clientset/versioned"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Topology Controllers Suite")
}

var (
	testEnv                   *envtest.Environment
	client                    runtimeClient.Client
	clientSet                 *topologyClient.Clientset
	ctx                       context.Context
	cancel                    context.CancelFunc
	mgr                       ctrl.Manager
	fakeRabbitMQClient        *rabbitmqclientfakes.FakeClient
	fakeRabbitMQClientError   error
	fakeRabbitMQClientFactory = func(connectionCreds map[string]string, tlsEnabled bool, certPool *x509.CertPool) (rabbitmqclient.Client, error) {
		fakeRabbitMQClientFactoryArgsForCall = append(fakeRabbitMQClientFactoryArgsForCall, struct {
			arg1 map[string]string
			arg2 bool
			arg3 *x509.CertPool
		}{connectionCreds, tlsEnabled, certPool})
		return fakeRabbitMQClient, fakeRabbitMQClientError
	}
	// Shameless copy of what counterfeiter does for mocking
	fakeRabbitMQClientFactoryArgsForCall []struct {
		arg1 map[string]string
		arg2 bool
		arg3 *x509.CertPool
	}
	fakeRecorder              *record.FakeRecorder
	topologyReconcilers       []*topologycontrollers.TopologyReconciler
	superStreamReconciler     *topologycontrollers.SuperStreamReconciler
	statusEventsUpdateTimeout = 10 * time.Second
)

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))

	ctx, cancel = context.WithCancel(ctrl.SetupSignalHandler())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	Expect(scheme.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(rabbitmqv1beta1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(rabbitmqv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())

	clientSet, err = topologyClient.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())

	mgr, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0", // To avoid macOS firewall pop-up every time you run this suite
	})
	Expect(err).ToNot(HaveOccurred())

	fakeRecorder = record.NewFakeRecorder(128)

	// The order in which these are declared matters
	// Keep it sync with the order in which 'topologyObjects' are declared in 'common_test.go`
	topologyReconcilers = []*topologycontrollers.TopologyReconciler{
		{
			Client:                mgr.GetClient(),
			Type:                  &rabbitmqv1beta1.Binding{},
			Scheme:                mgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &topologycontrollers.BindingReconciler{},
		},
		{
			Client:                mgr.GetClient(),
			Type:                  &rabbitmqv1beta1.Exchange{},
			Scheme:                mgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &topologycontrollers.ExchangeReconciler{},
		},
		{
			Client:                mgr.GetClient(),
			Type:                  &rabbitmqv1beta1.Permission{},
			Scheme:                mgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &topologycontrollers.PermissionReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()},
		},
		{
			Client:                mgr.GetClient(),
			Type:                  &rabbitmqv1beta1.Policy{},
			Scheme:                mgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &topologycontrollers.PolicyReconciler{},
		},
		{
			Client:                mgr.GetClient(),
			Type:                  &rabbitmqv1beta1.Queue{},
			Scheme:                mgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &topologycontrollers.QueueReconciler{},
		},
		{
			Client:                mgr.GetClient(),
			Type:                  &rabbitmqv1beta1.User{},
			Scheme:                mgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &topologycontrollers.UserReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()},
		},
		{
			Client:                mgr.GetClient(),
			Type:                  &rabbitmqv1beta1.Vhost{},
			Scheme:                mgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &topologycontrollers.VhostReconciler{Client: mgr.GetClient()},
		},
		{
			Client:                mgr.GetClient(),
			Type:                  &rabbitmqv1beta1.SchemaReplication{},
			Scheme:                mgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &topologycontrollers.SchemaReplicationReconciler{Client: mgr.GetClient()},
		},
		{
			Client:                mgr.GetClient(),
			Type:                  &rabbitmqv1beta1.Federation{},
			Scheme:                mgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &topologycontrollers.FederationReconciler{Client: mgr.GetClient()},
		},
		{
			Client:                mgr.GetClient(),
			Type:                  &rabbitmqv1beta1.Shovel{},
			Scheme:                mgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &topologycontrollers.ShovelReconciler{Client: mgr.GetClient()},
		},
		{
			Client:                mgr.GetClient(),
			Type:                  &rabbitmqv1beta1.TopicPermission{},
			Scheme:                mgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &topologycontrollers.TopicPermissionReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()},
		},
	}

	for _, controller := range topologyReconcilers {
		Expect(controller.SetupWithManager(mgr)).To(Succeed())
	}

	superStreamReconciler = &topologycontrollers.SuperStreamReconciler{
		Client:                mgr.GetClient(),
		Scheme:                mgr.GetScheme(),
		Recorder:              fakeRecorder,
		RabbitmqClientFactory: fakeRabbitMQClientFactory,
	}

	Expect(superStreamReconciler.SetupWithManager(mgr)).To(Succeed())

	go func() {
		err = mgr.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

	client = mgr.GetClient()
	Expect(client).ToNot(BeNil())

	komega.SetClient(client)
	komega.SetContext(ctx)

	rmqCreds := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-rabbit-user-credentials",
			Namespace: "default",
		},
	}
	Expect(client.Create(ctx, &rmqCreds)).To(Succeed())

	rmqSrv := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-rabbit",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port: 15672,
					Name: "management",
				},
				{
					Port: 15671,
					Name: "management-tls",
				},
			},
		},
	}
	Expect(client.Create(ctx, &rmqSrv)).To(Succeed())

	rmq := rabbitmqv1beta1.RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-rabbit",
			Namespace: "default",
			Annotations: map[string]string{
				"rabbitmq.com/topology-allowed-namespaces": "allowed",
			},
		},
		Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
			TLS: rabbitmqv1beta1.TLSSpec{
				SecretName: "i-do-not-exist-but-its-fine",
			},
		},
	}
	Expect(client.Create(ctx, &rmq)).To(Succeed())

	rmq.Status = rabbitmqv1beta1.RabbitmqClusterStatus{
		Binding: &corev1.LocalObjectReference{
			Name: "example-rabbit-user-credentials",
		},
		DefaultUser: &rabbitmqv1beta1.RabbitmqClusterDefaultUser{
			ServiceReference: &rabbitmqv1beta1.RabbitmqClusterServiceReference{
				Name:      "example-rabbit",
				Namespace: "default",
			},
		},
	}
	rmq.Status.SetConditions([]runtime.Object{})
	Expect(client.Status().Update(ctx, &rmq)).To(Succeed())

	rmqCreds = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-all-rabbit-user-credentials",
			Namespace: "default",
		},
	}
	Expect(client.Create(ctx, &rmqCreds)).To(Succeed())

	rmqSrv = corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-all-rabbit",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port: 15672,
					Name: "management",
				},
				{
					Port: 15671,
					Name: "management-tls",
				},
			},
		},
	}
	Expect(client.Create(ctx, &rmqSrv)).To(Succeed())

	rmq = rabbitmqv1beta1.RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-all-rabbit",
			Namespace: "default",
			Annotations: map[string]string{
				"rabbitmq.com/topology-allowed-namespaces": "*",
			},
		},
	}
	Expect(client.Create(ctx, &rmq)).To(Succeed())

	rmq.Status = rabbitmqv1beta1.RabbitmqClusterStatus{
		Binding: &corev1.LocalObjectReference{
			Name: "allow-all-rabbit-user-credentials",
		},
		DefaultUser: &rabbitmqv1beta1.RabbitmqClusterDefaultUser{
			ServiceReference: &rabbitmqv1beta1.RabbitmqClusterServiceReference{
				Name:      "allow-all-rabbit",
				Namespace: "default",
			},
		},
	}
	rmq.Status.SetConditions([]runtime.Object{})
	Expect(client.Status().Update(ctx, &rmq)).To(Succeed())

	allowedNamespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "allowed",
		},
	}
	Expect(client.Create(ctx, &allowedNamespace)).To(Succeed())

	prohibitedNamespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "prohibited",
		},
	}
	Expect(client.Create(ctx, &prohibitedNamespace)).To(Succeed())

	endpointsSecretBody := map[string][]byte{
		"username":  []byte("a-random-user"),
		"password":  []byte("a-random-password"),
		"endpoints": []byte("a.endpoints.local:5672,b.endpoints.local:5672,c.endpoints.local:5672"),
	}

	// used in schema-replication-controller test
	endpointsSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "endpoints-secret",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
		Data: endpointsSecretBody,
	}
	Expect(client.Create(ctx, &endpointsSecret)).To(Succeed())

	allowedEndpointsSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "endpoints-secret",
			Namespace: "allowed",
		},
		Type: corev1.SecretTypeOpaque,
		Data: endpointsSecretBody,
	}
	Expect(client.Create(ctx, &allowedEndpointsSecret)).To(Succeed())

	prohibitedEndpointsSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "endpoints-secret",
			Namespace: "prohibited",
		},
		Type: corev1.SecretTypeOpaque,
		Data: endpointsSecretBody,
	}
	Expect(client.Create(ctx, &prohibitedEndpointsSecret)).To(Succeed())

	federationUriSecretBody := map[string][]byte{
		"uri": []byte("amqp://rabbit@rabbit:a-rabbitmq-uri.test.com"),
	}

	// used in federation-controller test
	federationUriSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "federation-uri",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
		Data: federationUriSecretBody,
	}
	Expect(client.Create(ctx, &federationUriSecret)).To(Succeed())

	allowedFederationUriSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "federation-uri",
			Namespace: "allowed",
		},
		Type: corev1.SecretTypeOpaque,
		Data: federationUriSecretBody,
	}
	Expect(client.Create(ctx, &allowedFederationUriSecret)).To(Succeed())

	prohibitedFederationUriSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "federation-uri",
			Namespace: "prohibited",
		},
		Type: corev1.SecretTypeOpaque,
		Data: federationUriSecretBody,
	}
	Expect(client.Create(ctx, &prohibitedFederationUriSecret)).To(Succeed())

	shovelUriSecretBody := map[string][]byte{
		"srcUri":  []byte("amqp://rabbit@rabbit:a-rabbitmq-uri.test.com"),
		"destUri": []byte("amqp://rabbit@rabbit:a-rabbitmq-uri.test.com"),
	}

	// used in shovel-controller test
	shovelUriSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shovel-uri-secret",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
		Data: shovelUriSecretBody,
	}
	Expect(client.Create(ctx, &shovelUriSecret)).To(Succeed())

	allowedShovelUriSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shovel-uri-secret",
			Namespace: "allowed",
		},
		Type: corev1.SecretTypeOpaque,
		Data: shovelUriSecretBody,
	}
	Expect(client.Create(ctx, &allowedShovelUriSecret)).To(Succeed())

	prohibitedShovelUriSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shovel-uri-secret",
			Namespace: "prohibited",
		},
		Type: corev1.SecretTypeOpaque,
		Data: shovelUriSecretBody,
	}
	Expect(client.Create(ctx, &prohibitedShovelUriSecret)).To(Succeed())
})

var _ = BeforeEach(func() {
	fakeRabbitMQClient = &rabbitmqclientfakes.FakeClient{}
	fakeRabbitMQClientError = nil
	fakeRabbitMQClientFactoryArgsForCall = nil
})

var _ = AfterEach(func() {
	for len(fakeRecorder.Events) > 0 {
		// Drain any unused events
		<-fakeRecorder.Events
	}
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	Expect(testEnv.Stop()).To(Succeed())
})

func observedEvents() []string {
	var events []string
	for len(fakeRecorder.Events) > 0 {
		events = append(events, <-fakeRecorder.Events)
	}
	return events
}

func FakeRabbitMQClientFactoryArgsForCall(i int) (map[string]string, bool, *x509.CertPool) {
	// More shameless copy of counterfeiter code generation idea
	argsForCall := fakeRabbitMQClientFactoryArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}
