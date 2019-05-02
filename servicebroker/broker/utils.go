package broker

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func clientsetConfig() (*rest.Config, error) {
	var err error
	var config *rest.Config
	if len(os.Getenv("KUBERNETES_SERVICE_HOST")) > 0 {
		// creates the in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("Failed to create in-cluster config: %s", err)
		}
	} else {
		var kubeconfig string
		if len(os.Getenv("KUBECONFIG")) > 0 {
			kubeconfig = os.Getenv("KUBECONFIG")
		} else {
			kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube/config")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("Failed to create out of cluster config: %s", err)
		}
	}

	return config, nil
}
