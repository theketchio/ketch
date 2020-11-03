package chart

import (
	"log"
	"os"

	"helm.sh/helm/v3/pkg/action"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"helm.sh/helm/v3/pkg/chart/loader"
	ctrl "sigs.k8s.io/controller-runtime"
)

// HelmClient performs helm install and uninstall operations for provided application helm charts.
type HelmClient struct {
	cfg       *action.Configuration
	namespace string
}

// NewHelmClient returns a HelmClient instance.
func NewHelmClient(namespace string) (*HelmClient, error) {
	cfg, err := getActionConfig(namespace)
	if err != nil {
		return nil, err
	}
	return &HelmClient{cfg: cfg, namespace: namespace}, nil
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

// UpdateChart checks if the app chart is already installed and performs "helm install" or "helm update" operation.
func (c HelmClient) UpdateChart(appChrt ApplicationChart, config ChartConfig) error {
	appName := appChrt.AppName()
	files, err := appChrt.bufferedFiles(config)
	if err != nil {
		return err
	}
	chrt, err := loader.LoadFiles(files)
	if err != nil {
		return err
	}
	vals, err := appChrt.getValues()
	if err != nil {
		return err
	}
	getValuesClient := action.NewGetValues(c.cfg)
	getValuesClient.AllValues = true
	_, err = getValuesClient.Run(appName)
	if err != nil && err.Error() == "release: not found" {
		clientInstall := action.NewInstall(c.cfg)
		clientInstall.ReleaseName = appName
		clientInstall.Namespace = c.namespace
		_, err := clientInstall.Run(chrt, vals)
		return err
	}
	if err != nil {
		return err
	}
	updateClient := action.NewUpgrade(c.cfg)
	updateClient.Namespace = c.namespace
	_, err = updateClient.Run(appName, chrt, vals)
	return err
}

// DeleteChart uninstalls the app's helm release. It doesn't return an error if the release is not found.
func (c HelmClient) DeleteChart(appName string) error {
	uninstall := action.NewUninstall(c.cfg)
	_, err := uninstall.Run(appName)
	if err != nil && err.Error() == "release: not found" {
		return nil
	}
	return err
}
