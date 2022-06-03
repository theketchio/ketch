package chart

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	helmTime "helm.sh/helm/v3/pkg/time"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type statusFunc func(cfg *action.Configuration, appName string) (*release.Release, release.Status, error)

const (
	waitRetry = iota
	takeAction
	noAction

	notFound release.Status = "not-found"
)

// helmStatusActionMapUpdate maps a Release Status to a Ketch action for helm updates
var helmStatusActionMapUpdate = map[release.Status]int{
	notFound:                      takeAction,
	release.StatusUnknown:         waitRetry,
	release.StatusDeployed:        takeAction,
	release.StatusUninstalled:     takeAction,
	release.StatusSuperseded:      noAction,
	release.StatusFailed:          takeAction,
	release.StatusUninstalling:    waitRetry,
	release.StatusPendingInstall:  waitRetry,
	release.StatusPendingUpgrade:  waitRetry,
	release.StatusPendingRollback: waitRetry,
}

// helmStatusActionMapDelete maps a Release Status to a Ketch action for helm deletions
var helmStatusActionMapDelete = map[release.Status]int{
	notFound:                      noAction,
	release.StatusUnknown:         waitRetry,
	release.StatusDeployed:        takeAction,
	release.StatusUninstalled:     noAction,
	release.StatusSuperseded:      noAction,
	release.StatusFailed:          noAction,
	release.StatusUninstalling:    noAction,
	release.StatusPendingInstall:  waitRetry,
	release.StatusPendingUpgrade:  waitRetry,
	release.StatusPendingRollback: waitRetry,
}

const (
	defaultDeploymentTimeout = 10 * time.Minute
)

// HelmClient performs helm install and uninstall operations for provided application helm charts.
type HelmClient struct {
	cfg        *action.Configuration
	namespace  string
	c          client.Client
	log        logr.Logger
	statusFunc statusFunc
}

// TemplateValuer is an interface that permits types that implement it (e.g. Application, Job)
// to be parameters in the UpdateChart function.
type TemplateValuer interface {
	GetValues() interface{}
	GetTemplates() map[string]string
	GetName() string
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
			log:                c.log,
			cli:                c.c,
			namespace:          c.namespace,
			appName:            config.AppName,
			deploymentVersions: config.DeploymentVersions,
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

	// MaxHistory specifies the maximum number of historical releases that will be retained, including the most recent release.
	// Values of 0 or less are ignored (meaning no limits are imposed).
	// Let's set it to minimal to disable "helm rollback".
	updateClient.MaxHistory = 1
	updateClient.PostRenderer = &postRender{
		cli:                c.c,
		log:                c.log,
		namespace:          c.namespace,
		appName:            config.AppName,
		deploymentVersions: config.DeploymentVersions,
	}
	shouldUpdate, err := c.isHelmChartStatusActionable(c.statusFunc, appName, helmStatusActionMapUpdate)
	if err != nil || !shouldUpdate {
		return nil, err
	}
	return updateClient.Run(appName, chrt, vals)
}

// DeleteChart uninstalls the app's helm release. It doesn't return an error if the release is not found.
func (c HelmClient) DeleteChart(appName string) error {
	shouldDelete, err := c.isHelmChartStatusActionable(c.statusFunc, appName, helmStatusActionMapDelete)
	if err != nil || !shouldDelete {
		return err
	}
	uninstall := action.NewUninstall(c.cfg)
	_, err = uninstall.Run(appName)
	if err != nil && errors.Is(err, driver.ErrReleaseNotFound) {
		return nil
	}
	return err
}

// getHelmStatus returns the latest Release, Status, and error for an app
func getHelmStatus(cfg *action.Configuration, appName string) (*release.Release, release.Status, error) {
	statusClient := action.NewStatus(cfg)
	status, err := statusClient.Run(appName)
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) || status.Info == nil {
			return nil, notFound, nil
		}
		return nil, "", err
	}
	return status, status.Info.Status, nil
}

// isHelmChartStatusActionable returns true if the statusFunc returns an actionable status according to the statusActionMap, false if the status is
// non-actionable (e.g. "not-found" status for a "delete" action), and an error if the status requires a wait-retry. The retry is expected to be
// executed by the calling reconciler's inherent looping.
func (c HelmClient) isHelmChartStatusActionable(statusFunc statusFunc, appName string, statusActionMap map[release.Status]int) (bool, error) {
	lastRelease, status, err := statusFunc(c.cfg, appName)
	if err != nil {
		return false, err
	}
	switch statusActionMap[status] {
	case noAction:
		c.log.Info(fmt.Sprintf("helm chart for app %s release already in state %s - no action required", appName, status))
		return false, nil
	case takeAction:
		return true, nil
	default:
		c.log.Info(fmt.Sprintf("Found pending helm release: %d", lastRelease.Version))
		timeoutLimit := time.Now().Add(-defaultDeploymentTimeout)
		if lastRelease.Info.FirstDeployed.Before(helmTime.Time{Time: timeoutLimit}) {
			newStatus := release.StatusDeployed
			c.log.Info(fmt.Sprintf("Setting status of release that has timeouted to: %s", newStatus))
			lastRelease.SetStatus(newStatus, "manually canceled")
			if err := c.cfg.Releases.Update(lastRelease); err != nil {
				return false, err
			}
			return true, nil
		}
		return false, fmt.Errorf("helm chart for app %s in non-actionable status %s", appName, status)
	}
}
