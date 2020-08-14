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
	"os"
	"strconv"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"github.com/rabbitmq/cluster-operator/controllers"
	"k8s.io/apimachinery/pkg/runtime"
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
	_ = defaultscheme.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	flag.StringVar(&metricsAddr, "metrics-addr", ":12345", "The address the metric endpoint binds to.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	operatorNamespace := os.Getenv("OPERATOR_NAMESPACE")
	if operatorNamespace == "" {
		log.Info("unable to find operator namespace")
		os.Exit(1)
	}

	// If the environment variable is not set Getenv returns an empty string which ctrl.Options.Namespace takes to mean all namespaces should be watched
	operatorScopeNamespace := os.Getenv("OPERATOR_SCOPE_NAMESPACE")

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

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	var clusterConfig *rest.Config
	if kubeConfigPath := os.Getenv("KUBE_CONFIG"); kubeConfigPath != "" {
		clusterConfig, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	} else {
		clusterConfig, err = rest.InClusterConfig()
	}

	if err != nil {
		log.Error(err, "unable to get kubernetes cluster config")
		os.Exit(1)
	}

	err = (&controllers.RabbitmqClusterReconciler{
		Client:        mgr.GetClient(),
		Log:           ctrl.Log.WithName(controllerName),
		Scheme:        mgr.GetScheme(),
		Recorder:      mgr.GetEventRecorderFor(controllerName),
		Namespace:     operatorNamespace,
		ClusterConfig: clusterConfig,
		Clientset:     kubernetes.NewForConfigOrDie(clusterConfig),
	}).SetupWithManager(mgr)
	if err != nil {
		log.Error(err, "unable to create controller", controllerName)
		os.Exit(1)
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
