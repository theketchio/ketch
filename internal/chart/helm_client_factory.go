package chart

import (
	"log"
	"os"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HelmClientFactory provides "NewHelmClient()" method to get a helm client.
// HelmClientFactory internally maintains a cache for action.Configurations per k8s namespace
// decreasing the cost of creating a new helm client.
type HelmClientFactory struct {
	sync.Mutex
	configurations              map[string]*action.Configuration // map[namespaceName]*action.Configuration
	configurationsLastUsedTimes map[string]time.Time

	lastCleanupTime time.Time

	getActionConfig func(namespace string) (*action.Configuration, error)
}

func NewHelmClientFactory() *HelmClientFactory {
	return &HelmClientFactory{
		configurations:              map[string]*action.Configuration{},
		configurationsLastUsedTimes: map[string]time.Time{},
		getActionConfig:             getActionConfig,
	}
}

// NewHelmClient returns a HelmClient instance.
func (f *HelmClientFactory) NewHelmClient(namespace string, c client.Client, log logr.Logger) (*HelmClient, error) {
	f.Lock()
	defer f.Unlock()

	f.cleanup()

	cfg, ok := f.configurations[namespace]
	if !ok {
		var err error
		cfg, err = f.getActionConfig(namespace)
		if err != nil {
			return nil, err
		}
		f.configurations[namespace] = cfg
	}
	f.configurationsLastUsedTimes[namespace] = time.Now()
	return &HelmClient{cfg: cfg, namespace: namespace, c: c, log: log.WithValues("helm-client", namespace), statusFunc: getHelmStatus}, nil
}

func (f *HelmClientFactory) cleanup() {
	if time.Since(f.lastCleanupTime) < 10*time.Minute {
		return
	}
	for ns, timestamp := range f.configurationsLastUsedTimes {
		if timestamp.Before(f.lastCleanupTime) {
			delete(f.configurations, ns)
			delete(f.configurationsLastUsedTimes, ns)
		}
	}
	f.lastCleanupTime = time.Now()
}

func getActionConfig(namespace string) (*action.Configuration, error) {
	actionConfig := new(action.Configuration)

	config := ctrl.GetConfigOrDie()

	// Create the ConfigFlags struct instance with initialized values from ServiceAccount
	kubeConfig := genericclioptions.NewConfigFlags(false)
	kubeConfig.APIServer = &config.Host
	kubeConfig.BearerToken = &config.BearerToken
	kubeConfig.CAFile = &config.CAFile
	kubeConfig.Namespace = &namespace
	if err := actionConfig.Init(kubeConfig, namespace, os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return nil, err
	}
	return actionConfig, nil
}
