// RabbitMQ Cluster Operator
//
// Copyright 2020 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Mozilla Public license, Version 2.0 (the "License").  You may not use this product except in compliance with the Mozilla Public License.
//
// This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	rabbitmqv1alpha1 "github.com/rabbitmq/cluster-operator/api/v1alpha1"
	"github.com/rabbitmq/cluster-operator/controllers/clustercontrollers"
	"github.com/rabbitmq/cluster-operator/controllers/topologycontrollers"
	"github.com/rabbitmq/cluster-operator/internal/rabbitmqclient"
	"github.com/rabbitmq/cluster-operator/pkg/profiling"
	corev1 "k8s.io/api/core/v1"
	"os"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

const controllerName = "rabbitmqcluster-controller"

var (
	scheme = runtime.NewScheme()
	log    = ctrl.Log.WithName("setup")
)

func init() {
	_ = rabbitmqv1beta1.AddToScheme(scheme)
	_ = rabbitmqv1alpha1.AddToScheme(scheme)
	_ = defaultscheme.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func sanitizeClusterDomainInput(clusterDomain string) string {
	if len(clusterDomain) == 0 {
		return ""
	}

	match, _ := regexp.MatchString("^\\.?[a-z]([-a-z0-9]*[a-z0-9])?(\\.[a-z]([-a-z0-9]*[a-z0-9])?)*$", clusterDomain) // Allow-list expression
	if !match {
		log.V(1).Info("Domain name value is invalid. Only alphanumeric characters, hyphens and dots are allowed.",
			topologycontrollers.KubernetesInternalDomainEnvVar, clusterDomain)
		return ""
	}

	if !strings.HasPrefix(clusterDomain, ".") {
		return fmt.Sprintf(".%s", clusterDomain)
	}

	return clusterDomain
}

func main() {
	var (
		metricsAddr             string
		defaultRabbitmqImage    = "rabbitmq:3.10.2-management"
		defaultUserUpdaterImage = "rabbitmqoperator/default-user-credential-updater:1.0.2"
		defaultImagePullSecrets = ""
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":9782", "The address the metric endpoint binds to.")

	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)

	flag.Parse()

	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)
	// https://github.com/kubernetes-sigs/controller-runtime/issues/1420#issuecomment-794525248
	klog.SetLogger(logger.WithName("rabbitmq-cluster-operator"))

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

	if configuredDefaultImagePullSecrets, ok := os.LookupEnv("DEFAULT_IMAGE_PULL_SECRETS"); ok {
		defaultImagePullSecrets = configuredDefaultImagePullSecrets
	}

	clusterDomain := sanitizeClusterDomainInput(os.Getenv(topologycontrollers.KubernetesInternalDomainEnvVar))

	options := ctrl.Options{
		Scheme:                  scheme,
		MetricsBindAddress:      metricsAddr,
		LeaderElection:          true,
		LeaderElectionNamespace: operatorNamespace,
		LeaderElectionID:        "rabbitmq-cluster-operator-leader-election",
		Namespace:               operatorScopeNamespace,
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

	if syncPeriod := os.Getenv(topologycontrollers.ControllerSyncPeriodEnvVar); syncPeriod != "" {
		syncPeriodDuration, err := time.ParseDuration(syncPeriod)
		if err != nil {
			log.Error(err, "unable to parse provided sync period", "sync period", syncPeriod)
			os.Exit(1)
		}
		options.SyncPeriod = &syncPeriodDuration
		log.Info(fmt.Sprintf("sync period set; all resources will be reconciled every: %s", syncPeriodDuration))
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if enableDebugPprof, ok := os.LookupEnv("ENABLE_DEBUG_PPROF"); ok {
		pprofEnabled, err := strconv.ParseBool(enableDebugPprof)
		if err == nil && pprofEnabled {
			mgr, err = profiling.AddDebugPprofEndpoints(mgr)
			if err != nil {
				log.Error(err, "unable to add debug endpoints to manager")
				os.Exit(1)
			}
		}
	}

	clusterConfig := config.GetConfigOrDie()

	err = (&clustercontrollers.RabbitmqClusterReconciler{
		Client:                  mgr.GetClient(),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(controllerName),
		Namespace:               operatorNamespace,
		ClusterConfig:           clusterConfig,
		Clientset:               kubernetes.NewForConfigOrDie(clusterConfig),
		PodExecutor:             clustercontrollers.NewPodExecutor(),
		DefaultRabbitmqImage:    defaultRabbitmqImage,
		DefaultUserUpdaterImage: defaultUserUpdaterImage,
		DefaultImagePullSecrets: defaultImagePullSecrets,
	}).SetupWithManager(mgr)
	if err != nil {
		log.Error(err, "unable to create controller", controllerName)
		os.Exit(1)
	}

	if err = (&topologycontrollers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &rabbitmqv1beta1.Queue{},
		Log:                     ctrl.Log.WithName(topologycontrollers.QueueControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(topologycontrollers.QueueControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &topologycontrollers.QueueReconciler{},
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", topologycontrollers.QueueControllerName)
		os.Exit(1)
	}

	if err = (&topologycontrollers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &rabbitmqv1beta1.Exchange{},
		Log:                     ctrl.Log.WithName(topologycontrollers.ExchangeControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(topologycontrollers.ExchangeControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &topologycontrollers.ExchangeReconciler{},
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", topologycontrollers.ExchangeControllerName)
		os.Exit(1)
	}

	if err = (&topologycontrollers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &rabbitmqv1beta1.Binding{},
		Log:                     ctrl.Log.WithName(topologycontrollers.BindingControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(topologycontrollers.BindingControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &topologycontrollers.BindingReconciler{},
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", topologycontrollers.BindingControllerName)
		os.Exit(1)
	}

	if err = (&topologycontrollers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &rabbitmqv1beta1.User{},
		Log:                     ctrl.Log.WithName(topologycontrollers.UserControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(topologycontrollers.UserControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		WatchTypes:              []client.Object{&corev1.Secret{}},
		ReconcileFunc:           &topologycontrollers.UserReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()},
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", topologycontrollers.UserControllerName)
		os.Exit(1)
	}

	if err = (&topologycontrollers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &rabbitmqv1beta1.Vhost{},
		Log:                     ctrl.Log.WithName(topologycontrollers.VhostControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(topologycontrollers.VhostControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &topologycontrollers.VhostReconciler{Client: mgr.GetClient()},
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", topologycontrollers.VhostControllerName)
		os.Exit(1)
	}

	if err = (&topologycontrollers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &rabbitmqv1beta1.Policy{},
		Log:                     ctrl.Log.WithName(topologycontrollers.PolicyControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(topologycontrollers.PolicyControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &topologycontrollers.PolicyReconciler{},
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", topologycontrollers.PolicyControllerName)
		os.Exit(1)
	}

	if err = (&topologycontrollers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &rabbitmqv1beta1.Permission{},
		Log:                     ctrl.Log.WithName(topologycontrollers.PermissionControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(topologycontrollers.PermissionControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &topologycontrollers.PermissionReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()},
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", topologycontrollers.PermissionControllerName)
		os.Exit(1)
	}

	if err = (&topologycontrollers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &rabbitmqv1beta1.SchemaReplication{},
		Log:                     ctrl.Log.WithName(topologycontrollers.SchemaReplicationControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(topologycontrollers.SchemaReplicationControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &topologycontrollers.SchemaReplicationReconciler{Client: mgr.GetClient()},
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", topologycontrollers.SchemaReplicationControllerName)
		os.Exit(1)
	}

	if err = (&topologycontrollers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &rabbitmqv1beta1.Federation{},
		Log:                     ctrl.Log.WithName(topologycontrollers.FederationControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(topologycontrollers.FederationControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &topologycontrollers.FederationReconciler{Client: mgr.GetClient()},
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", topologycontrollers.FederationControllerName)
		os.Exit(1)
	}

	if err = (&topologycontrollers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &rabbitmqv1beta1.Shovel{},
		Log:                     ctrl.Log.WithName(topologycontrollers.ShovelControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(topologycontrollers.ShovelControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &topologycontrollers.ShovelReconciler{Client: mgr.GetClient()},
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", topologycontrollers.ShovelControllerName)
		os.Exit(1)
	}

	if err = (&topologycontrollers.TopologyReconciler{
		Client:                  mgr.GetClient(),
		Type:                    &rabbitmqv1beta1.TopicPermission{},
		Log:                     ctrl.Log.WithName(topologycontrollers.TopicPermissionControllerName),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor(topologycontrollers.TopicPermissionControllerName),
		RabbitmqClientFactory:   rabbitmqclient.RabbitholeClientFactory,
		KubernetesClusterDomain: clusterDomain,
		ReconcileFunc:           &topologycontrollers.TopicPermissionReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()},
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", topologycontrollers.TopicPermissionControllerName)
		os.Exit(1)
	}

	if err = (&topologycontrollers.SuperStreamReconciler{
		Client:                mgr.GetClient(),
		Log:                   ctrl.Log.WithName(topologycontrollers.SuperStreamControllerName),
		Scheme:                mgr.GetScheme(),
		Recorder:              mgr.GetEventRecorderFor(topologycontrollers.SuperStreamControllerName),
		RabbitmqClientFactory: rabbitmqclient.RabbitholeClientFactory,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", topologycontrollers.SuperStreamControllerName)
		os.Exit(1)
	}

	if os.Getenv(topologycontrollers.EnableWebhooksEnvVar) != "false" {
		if err = (&rabbitmqv1beta1.Binding{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Binding")
			os.Exit(1)
		}
		if err = (&rabbitmqv1beta1.Queue{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Queue")
			os.Exit(1)
		}
		if err = (&rabbitmqv1beta1.Exchange{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Exchange")
			os.Exit(1)
		}
		if err = (&rabbitmqv1beta1.Vhost{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Vhost")
			os.Exit(1)
		}
		if err = (&rabbitmqv1beta1.Policy{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Policy")
			os.Exit(1)
		}
		if err = (&rabbitmqv1beta1.User{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "User")
			os.Exit(1)
		}
		if err = (&rabbitmqv1beta1.Permission{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Permission")
			os.Exit(1)
		}
		if err = (&rabbitmqv1beta1.SchemaReplication{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "SchemaReplication")
			os.Exit(1)
		}
		if err = (&rabbitmqv1beta1.Federation{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Federation")
			os.Exit(1)
		}
		if err = (&rabbitmqv1beta1.Shovel{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "Shovel")
			os.Exit(1)
		}
		if err = (&rabbitmqv1alpha1.SuperStream{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "SuperStream")
			os.Exit(1)
		}
		if err = (&rabbitmqv1beta1.TopicPermission{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "TopicPermission")
			os.Exit(1)
		}
	}

	log.Info("started controller")
	// +kubebuilder:scaffold:builder

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
