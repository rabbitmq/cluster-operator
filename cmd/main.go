// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.

package main

import (
	"crypto/fips140"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/rabbitmq/cluster-operator/v2/pkg/profiling"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	controllers "github.com/rabbitmq/cluster-operator/v2/internal/controller"
	"github.com/rabbitmq/cluster-operator/v2/internal/rabbitmqclient"
	webhookv1beta1 "github.com/rabbitmq/cluster-operator/v2/internal/webhook/v1beta1"
	// +kubebuilder:scaffold:imports
)

const controllerName = "rabbitmqcluster-controller"

var (
	scheme = runtime.NewScheme()
	log    = ctrl.Log.WithName("setup")
)

func init() {
	_ = rabbitmqv1beta1.AddToScheme(scheme)
	_ = defaultscheme.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		metricsAddr             string
		metricsCertPath         string
		metricsCertName         string
		metricsCertKey          string
		probeAddr               string
		secureMetrics           bool
		enableHTTP2             bool
		defaultRabbitmqImage    = "rabbitmq:4.2.6-management"
		controlRabbitmqImage    = false
		defaultUserUpdaterImage = "ghcr.io/rabbitmq/default-user-credential-updater:1.0.14"
		defaultImagePullSecrets = ""
		tlsOpts                 []func(*tls.Config)
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=true to enable HTTPS.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")

	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)

	flag.Parse()

	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)
	// https://github.com/kubernetes-sigs/controller-runtime/issues/1420#issuecomment-794525248
	klog.SetLogger(logger.WithName("rabbitmq-cluster-operator"))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		log.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	operatorNamespace := os.Getenv("OPERATOR_NAMESPACE")
	if operatorNamespace == "" {
		log.Info("unable to find operator namespace")
		os.Exit(1)
	}

	// If the environment variable is not set Getenv returns an empty string which ctrl.Options.Namespace takes to mean all namespaces should be watched
	operatorScopeNamespace := os.Getenv("OPERATOR_SCOPE_NAMESPACE")

	if configuredDefaultRabbitmqImage, ok := os.LookupEnv("DEFAULT_RABBITMQ_IMAGE"); ok {
		defaultRabbitmqImage = configuredDefaultRabbitmqImage
	}

	if configuredDefaultUserUpdaterImage, ok := os.LookupEnv("DEFAULT_USER_UPDATER_IMAGE"); ok {
		defaultUserUpdaterImage = configuredDefaultUserUpdaterImage
	}

	// EXPERIMENTAL: If the environment variable CONTROL_RABBITMQ_IMAGE is set to `true`, the operator will
	// automatically set the default image tags. (DEFAULT_RABBITMQ_IMAGE and DEFAULT_USER_UPDATER_IMAGE)
	// No safety checks!
	if configuredControlRabbitmqImage, ok := os.LookupEnv("CONTROL_RABBITMQ_IMAGE"); ok {
		var err error
		if controlRabbitmqImage, err = strconv.ParseBool(configuredControlRabbitmqImage); err != nil {
			log.Error(err, "unable to start manager")
			os.Exit(1)
		}
	}

	if configuredDefaultImagePullSecrets, ok := os.LookupEnv("DEFAULT_IMAGE_PULL_SECRETS"); ok {
		defaultImagePullSecrets = configuredDefaultImagePullSecrets
	}

	options := ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			CertDir:       metricsCertPath,
			CertName:      metricsCertName,
			KeyName:       metricsCertKey,
			TLSOpts:       tlsOpts,
		},
		HealthProbeBindAddress:  probeAddr,
		LeaderElection:          true,
		LeaderElectionNamespace: operatorNamespace,
		LeaderElectionID:        "rabbitmq-cluster-operator-leader-election",
		// Namespace is deprecated. Advice is to use Cache.Namespaces instead
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/metrics/filters#WithAuthenticationAndAuthorization
		options.Metrics.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	if operatorScopeNamespace != "" {
		if strings.Contains(operatorScopeNamespace, ",") {
			namespaces := strings.Split(operatorScopeNamespace, ",")
			// https://github.com/kubernetes-sigs/controller-runtime/blob/main/designs/cache_options.md#only-cache-namespaced-objects-in-the-foo-and-bar-namespace
			// Sometimes I wish that controller-runtime graduated to 1.x
			// This changed in 0.15, and again in 0.16 🤦
			options.Cache.DefaultNamespaces = make(map[string]cache.Config)
			for _, namespace := range namespaces {
				options.Cache.DefaultNamespaces[namespace] = cache.Config{}
			}
			log.Info("limiting watch to specific namespaces for RabbitMQ resources", "namespaces", namespaces)
		} else {
			options.Cache = cache.Options{
				DefaultNamespaces: map[string]cache.Config{operatorScopeNamespace: {}},
			}
			log.Info("limiting watch to one namespace", "namespace", operatorScopeNamespace)
		}
	}

	rmqLabel, err := labels.NewRequirement("app.kubernetes.io/part-of", selection.Equals, []string{"rabbitmq"})
	if err != nil {
		log.Error(err, "unable to create a label filter")
		os.Exit(1)
	}
	rmqSelector := labels.NewSelector().Add(*rmqLabel)

	options.Cache.ByObject = map[client.Object]cache.ByObject{
		&rabbitmqv1beta1.RabbitmqCluster{}: {},
		&appsv1.StatefulSet{}:              {Label: rmqSelector},
		&corev1.Service{}:                  {Label: rmqSelector},
		&corev1.ConfigMap{}:                {Label: rmqSelector},
		&corev1.Secret{}:                   {Label: rmqSelector},
		&corev1.ServiceAccount{}:           {Label: rmqSelector},
		&discoveryv1.EndpointSlice{}:       {Label: rmqSelector},
		&rbacv1.Role{}:                     {Label: rmqSelector},
		&rbacv1.RoleBinding{}:              {Label: rmqSelector},
	}

	if leaseDuration := getEnvInDuration("LEASE_DURATION"); leaseDuration != 0 {
		log.Info("manager configured with lease duration", "seconds", int(leaseDuration.Seconds()))
		options.LeaseDuration = &leaseDuration
	}

	if renewDeadline := getEnvInDuration("RENEW_DEADLINE"); renewDeadline != 0 {
		log.Info("manager configured with renew deadline", "seconds", int(renewDeadline.Seconds()))
		options.RenewDeadline = &renewDeadline
	}

	if retryPeriod := getEnvInDuration("RETRY_PERIOD"); retryPeriod != 0 {
		log.Info("manager configured with retry period", "seconds", int(retryPeriod.Seconds()))
		options.RetryPeriod = &retryPeriod
	}

	if enableDebugPprof, ok := os.LookupEnv("ENABLE_DEBUG_PPROF"); ok {
		pprofEnabled, err := strconv.ParseBool(enableDebugPprof)
		if err == nil && pprofEnabled {
			o, err := profiling.AddDebugPprofEndpoints(&options)
			if err != nil {
				log.Error(err, "unable to add debug endpoints to manager")
				os.Exit(1)
			}
			options = *o
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	clusterConfig := config.GetConfigOrDie()

	if err = (&controllers.RabbitmqClusterReconciler{
		Client:                  mgr.GetClient(),
		APIReader:               mgr.GetAPIReader(),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controllerName),
		Namespace:               operatorNamespace,
		ClusterConfig:           clusterConfig,
		Clientset:               kubernetes.NewForConfigOrDie(clusterConfig),
		PodExecutor:             controllers.NewPodExecutor(),
		DefaultRabbitmqImage:    defaultRabbitmqImage,
		DefaultUserUpdaterImage: defaultUserUpdaterImage,
		DefaultImagePullSecrets: defaultImagePullSecrets,
		ControlRabbitmqImage:    controlRabbitmqImage,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", controllerName)
		os.Exit(1)
	}

	if _, disabled := os.LookupEnv("DISABLE_DEPRECATED_FEATURES_CHECK"); !disabled {
		deprecatedFeaturesCheckInterval := 5 * time.Minute
		if envInterval := getEnvInDuration("DEPRECATED_FEATURES_CHECK_INTERVAL"); envInterval != 0 {
			deprecatedFeaturesCheckInterval = envInterval
		}

		if err = (&controllers.DeprecatedFeatureReconciler{
			Client:                mgr.GetClient(),
			APIReader:             mgr.GetAPIReader(),
			RabbitmqClientFactory: &rabbitmqclient.DefaultRabbitmqClientFactory{},
			Interval:              deprecatedFeaturesCheckInterval,
		}).SetupWithManager(mgr); err != nil {
			log.Error(err, "unable to create controller", "controller", "deprecated-feature-controller")
			os.Exit(1)
		}
	}
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err := webhookv1beta1.SetupRabbitmqClusterWebhookWithManager(mgr, webhookv1beta1.RabbitmqClusterCustomDefaulter{
			DefaultRabbitmqImage:    defaultRabbitmqImage,
			DefaultImagePullSecrets: defaultImagePullSecrets,
			DefaultUserUpdaterImage: defaultUserUpdaterImage,
		}); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "RabbitmqCluster")
			os.Exit(1)
		}
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	if fips140.Enabled() {
		log.Info("FIPS 140-3 mode enabled")
	}

	log.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func getEnvInDuration(envName string) time.Duration {
	var durationInt int64
	if durationStr := os.Getenv(envName); durationStr != "" {
		var err error
		if durationInt, err = strconv.ParseInt(durationStr, 10, 64); err != nil {
			log.Error(err, fmt.Sprintf("unable to parse provided '%s'", envName))
			os.Exit(1)
		}
	}
	return time.Duration(durationInt) * time.Second
}
