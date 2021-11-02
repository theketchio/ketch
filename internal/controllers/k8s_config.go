package controllers

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// GetRESTConfig returns a rest.Config. It uses the presence of KUBERNETES_SERVICE_HOST
// to determine whether to use an InClusterConfig or the user's config.
func GetRESTConfig() (*rest.Config, error) {
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		return externalConfig()
	}
	return rest.InClusterConfig()
}

// externalConfig returns a REST config to be run external to the cluster, e.g. testing locally.
func externalConfig() (*rest.Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configStr := filepath.Join(home, ".kube", "config")
	return clientcmd.BuildConfigFromFlags("", configStr)
}
