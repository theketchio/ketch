package chart

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	helmTime "helm.sh/helm/v3/pkg/time"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultDeploymentTimeout = 10 * time.Minute
)

// HelmClient performs helm install and uninstall operations for provided application helm charts.
type HelmClient struct {
	cfg       *action.Configuration
	namespace string
	c         client.Client
	log       logr.Logger
}

// TemplateValuer is an interface that permits types that implement it (e.g. Application, Job)
// to be parameters in the UpdateChart function.
type TemplateValuer interface {
	GetValues() interface{}
	GetTemplates() map[string]string
	GetName() string
}

// NewHelmClient returns a HelmClient instance.
func NewHelmClient(namespace string, c client.Client, log logr.Logger) (*HelmClient, error) {
	cfg, err := getActionConfig(namespace)
	if err != nil {
		return nil, err
	}
	return &HelmClient{cfg: cfg, namespace: namespace, c: c, log: log.WithValues("helm-client", namespace)}, nil
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

// InstallOption to perform additional configuration of action.Install before running a chart installation.
type InstallOption func(install *action.Install)

// UpdateChart checks if the app chart is already installed and performs "helm install" or "helm update" operation.
func (c HelmClient) UpdateChart(tv TemplateValuer, config ChartConfig, opts ...InstallOption) (*release.Release, error) {
	appName := tv.GetName()
	files, err := bufferedFiles(config, tv.GetTemplates(), tv.GetValues())
	if err != nil {
		return nil, err
	}
	chrt, err := loader.LoadFiles(files)
	if err != nil {
		return nil, err
	}
	vals, err := getValuesMap(tv.GetValues())
	if err != nil {
		return nil, err
	}
	getValuesClient := action.NewGetValues(c.cfg)
	getValuesClient.AllValues = true
	_, err = getValuesClient.Run(appName)
	if err != nil && errors.Is(err, driver.ErrReleaseNotFound) {
		clientInstall := action.NewInstall(c.cfg)
		clientInstall.ReleaseName = appName
		clientInstall.Namespace = c.namespace
		clientInstall.PostRenderer = &postRender{
			namespace: c.namespace,
			cli:       c.c,
		}
		for _, opt := range opts {
			opt(clientInstall)
		}
		return clientInstall.Run(chrt, vals)
	}
	if err != nil {
		return nil, err
	}
	updateClient := action.NewUpgrade(c.cfg)
	updateClient.Namespace = c.namespace

	// we check releases and if lastRelease release was a while back, we set its status to Failure
	// next helm update won't be blocked in that case
	// this is an edge case we should cover when ketch controller pod gets restarted in middle of deploying and app
	// in that case if helm secret (release) remains it would block next deployment forever
	lastRelease, err := c.cfg.Releases.Last(appName)
	if err != nil && err != driver.ErrReleaseNotFound {
		return nil, err
	}
	if lastRelease != nil {
		if lastRelease.Info.Status.IsPending() {
			c.log.Info(fmt.Sprintf("Found pending helm release: %d", lastRelease.Version))
			timeoutLimit := time.Now().Add(-defaultDeploymentTimeout)
			if lastRelease.Info.FirstDeployed.Before(helmTime.Time{Time: timeoutLimit}) {
				newStatus := release.StatusDeployed
				c.log.Info(fmt.Sprintf("Setting status of release that has timeouted to: %s", newStatus))
				lastRelease.SetStatus(newStatus, "manually canceled")
				if err := c.cfg.Releases.Update(lastRelease); err != nil {
					return nil, err
				}
			}
		}
	}
	// MaxHistory specifies the maximum number of historical releases that will be retained, including the most recent release.
	// Values of 0 or less are ignored (meaning no limits are imposed).
	// Let's set it to minimal to disable "helm rollback".
	updateClient.MaxHistory = 1
	updateClient.PostRenderer = &postRender{
		namespace: c.namespace,
		cli:       c.c,
	}
	return updateClient.Run(appName, chrt, vals)
}

// DeleteChart uninstalls the app's helm release. It doesn't return an error if the release is not found.
func (c HelmClient) DeleteChart(appName string) error {
	uninstall := action.NewUninstall(c.cfg)
	_, err := uninstall.Run(appName)
	if err != nil && errors.Is(err, driver.ErrReleaseNotFound) {
		return nil
	}
	return err
}
