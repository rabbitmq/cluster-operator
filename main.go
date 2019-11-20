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

package main

import (
	"flag"
	"github.com/pivotal/rabbitmq-for-kubernetes/internal/resource"
	"io/ioutil"
	"os"

	"github.com/pivotal/rabbitmq-for-kubernetes/internal/config"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"github.com/pivotal/rabbitmq-for-kubernetes/controllers"
	"k8s.io/apimachinery/pkg/runtime"
	defaultscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {

	rabbitmqv1beta1.AddToScheme(scheme)
	defaultscheme.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	flag.StringVar(&metricsAddr, "metrics-addr", ":12345", "The address the metric endpoint binds to.")
	flag.Parse()

	ctrl.SetLogger(zap.Logger(true))

	operatorNamespace := os.Getenv("OPERATOR_NAMESPACE")
	if operatorNamespace == "" {
		setupLog.Info("Unable to find operator namespace")
		os.Exit(1)
	}

	options := ctrl.Options{
		Scheme:                  scheme,
		MetricsBindAddress:      metricsAddr,
		LeaderElection:          true,
		LeaderElectionNamespace: operatorNamespace,
		LeaderElectionID:        "pivotal-rabbitmq-operator-leader-election",
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	configPath := os.Getenv("CONFIG_FILEPATH")
	if configPath == "" {
		setupLog.Error(err, "unable to find config file")
		os.Exit(1)
	}
	rawConfig, err := ioutil.ReadFile(configPath)
	if err != nil {
		setupLog.Error(err, "unable to read config file")
		os.Exit(1)
	}

	config, err := config.NewConfig(rawConfig)
	if err != nil {
		setupLog.Error(err, "unable to parse config")
		os.Exit(1)
	}

	resourceRequirements := resource.ResourceRequirements{
		CPULimit:      config.Resources.Limits.CPU,
		MemoryLimit:   config.Resources.Limits.Memory,
		CPURequest:    config.Resources.Requests.CPU,
		MemoryRequest: config.Resources.Requests.Memory,
	}
	err = (&controllers.RabbitmqClusterReconciler{
		Client:                     mgr.GetClient(),
		Log:                        ctrl.Log.WithName("controllers").WithName("RabbitmqCluster"),
		Scheme:                     mgr.GetScheme(),
		ServiceType:                config.Service.Type,
		ServiceAnnotations:         config.Service.Annotations,
		Image:                      config.Image,
		ImagePullSecret:            config.ImagePullSecret,
		PersistentStorage:          config.Persistence.Storage,
		PersistentStorageClassName: config.Persistence.StorageClassName,
		ResourceRequirements:       resourceRequirements,
		Namespace:                  operatorNamespace,
	}).SetupWithManager(mgr)
	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RabbitmqCluster")
		os.Exit(1)
	}
	setupLog.Info("Started controller with ServiceType %s and ServiceAnnotation %s", config.Service.Type, config.Service.Annotations)
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
