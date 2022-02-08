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
	WaitRetry = iota
	TakeAction
	NoAction
)

// helmStatusActionMapUpdate maps a Release Status to a Ketch action for helm updates
var helmStatusActionMapUpdate = map[release.Status]int{
	"not-found":                   TakeAction,
	release.StatusUnknown:         WaitRetry,
	release.StatusDeployed:        TakeAction,
	release.StatusUninstalled:     TakeAction,
	release.StatusSuperseded:      NoAction,
	release.StatusFailed:          TakeAction,
	release.StatusUninstalling:    WaitRetry,
	release.StatusPendingInstall:  WaitRetry,
	release.StatusPendingUpgrade:  WaitRetry,
	release.StatusPendingRollback: WaitRetry,
}

// helmStatusActionMapDelete maps a Release Status to a Ketch action for helm deletions
var helmStatusActionMapDelete = map[release.Status]int{
	"not-found":                   NoAction,
	release.StatusUnknown:         WaitRetry,
	release.StatusDeployed:        TakeAction,
	release.StatusUninstalled:     NoAction,
	release.StatusSuperseded:      NoAction,
	release.StatusFailed:          NoAction,
	release.StatusUninstalling:    NoAction,
	release.StatusPendingInstall:  WaitRetry,
	release.StatusPendingUpgrade:  WaitRetry,
	release.StatusPendingRollback: WaitRetry,
}

var (
	statusRetryInterval = time.Second * 1
	statusRetryTimeout  = time.Second * 5
)

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
	shouldUpdate, err := c.waitForActionableStatus(c.statusFunc, appName, helmStatusActionMapUpdate)
	if err != nil || !shouldUpdate {
		return nil, err
	}
	return updateClient.Run(appName, chrt, vals)
}

// DeleteChart uninstalls the app's helm release. It doesn't return an error if the release is not found.
func (c HelmClient) DeleteChart(appName string) error {
	shouldDelete, err := c.waitForActionableStatus(c.statusFunc, appName, helmStatusActionMapDelete)
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

// getHelmStatus returns the Release, Status, and error for an app
func getHelmStatus(cfg *action.Configuration, appName string) (*release.Release, release.Status, error) {
	statusClient := action.NewStatus(cfg)
	status, err := statusClient.Run(appName)
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) || status.Info == nil {
			return nil, "not-found", nil
		}
		return nil, "", err
	}
	return status, status.Info.Status, nil
}

// waitForActionableStatus returns true if the helm status is such that it is ok to proceed with update/delete. If the statusFunc returns
// WaitRetry, the func blocks while retrying.
func (c HelmClient) waitForActionableStatus(statusFunc statusFunc, appName string, statusActionMap map[release.Status]int) (bool, error) {
	ticker := time.NewTicker(statusRetryInterval)
	done := time.After(statusRetryTimeout)
	var helmRelease *release.Release
	var status release.Status
	var err error
	for {
		select {
		case <-done:
			c.log.Info(fmt.Sprintf("Setting status (%s) of %s release that has timeouted to: deployed", appName, status))
			helmRelease.SetStatus(release.StatusDeployed, "set manually after timeout waiting for actionable status")
			return true, nil
		case <-ticker.C:
			helmRelease, status, err = statusFunc(c.cfg, appName)
			if err != nil {
				return false, err
			}
			action := statusActionMap[status] // default wait-retry
			if action == NoAction {
				c.log.Info(fmt.Sprintf("helm chart for app %s release already in state %s - no action required", appName, status))
				return false, nil
			}
			if action == TakeAction {
				return true, nil
			}
		}
	}
}
