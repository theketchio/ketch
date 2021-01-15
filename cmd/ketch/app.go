package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	apiv1 "k8s.io/api/core/v1"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

func newAppCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app",
		Short: "Manage applications",
		Long:  `Manage applications`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(newAppCreateCmd(cfg, out))
	cmd.AddCommand(newAppDeployCmd(cfg, out))
	cmd.AddCommand(newAppUpdateCmd(cfg, out))
	cmd.AddCommand(newAppListCmd(cfg, out))
	cmd.AddCommand(newAppLogCmd(cfg, out, appLog))
	cmd.AddCommand(newAppRemoveCmd(cfg, out, appRemove))
	cmd.AddCommand(newAppInfoCmd(cfg, out))
	cmd.AddCommand(newAppStartCmd(cfg, out, appStart))
	cmd.AddCommand(newAppStopCmd(cfg, out, appStop))
	cmd.AddCommand(newAppExportCmd(cfg, out, appExport))
	return cmd
}

func appState(pods []apiv1.Pod) (state string) {
	state = "unknown"
	// If there's no pods for this app, then app in `created` state
	if len(pods) == 0 {
		state = strings.ToLower(string(ketchv1.AppCreated))
		return state
	}

	// If app has pods, then we build app status based on pods statuses
	var deploying, running, errored, succeeded int
	for _, pod := range pods {
		switch podState(pod) {
		case ketchv1.PodRunning:
			running++
		case ketchv1.PodDeploying:
			deploying++
		case ketchv1.PodError:
			errored++
		case ketchv1.PodSucceeded:
			succeeded++
		}
	}

	var parts []string
	if deploying > 0 {
		parts = append(parts, fmt.Sprintf(`%d %s`, deploying, ketchv1.PodDeploying))
	}
	if running > 0 {
		parts = append(parts, fmt.Sprintf(`%d %s`, running, ketchv1.PodRunning))
	}
	if errored > 0 {
		parts = append(parts, fmt.Sprintf(`%d %s`, errored, ketchv1.PodError))
	}
	if succeeded > 0 {
		parts = append(parts, fmt.Sprintf(`%d %s`, succeeded, ketchv1.PodSucceeded))
	}

	return strings.Join(parts, ", ")
}

var (
	containerStateCrashLoopBackOff = "CrashLoopBackOff"
	containerStateCompleted        = "Completed"
)

func podState(pod apiv1.Pod) ketchv1.PodState {
	switch pod.Status.Phase {
	case apiv1.PodPending:
		return ketchv1.PodDeploying
	case apiv1.PodRunning:
		// In some cases, we have to override the status obtained by using `pod.Status.Phase` because it could be
		// `running` state while the actual container in `CrashLoopBackOff` so, actual state of the pod is `error`,
		// and so on.
		for _, c := range pod.Status.ContainerStatuses {
			// If any of the pod containers in the `CrashLoopBackOff` state we must return the `error` state
			if c.State.Waiting != nil {
				if c.State.Waiting.Reason == containerStateCrashLoopBackOff {
					return ketchv1.PodError
				}
			}
			// If any of the pod containers in the `Completed` state we must return the `succeeded` state
			if c.State.Terminated != nil {
				if c.State.Terminated.Reason == containerStateCompleted {
					return ketchv1.PodSucceeded
				}
			}
		}
		return ketchv1.PodRunning
	case apiv1.PodFailed, apiv1.PodUnknown:
		return ketchv1.PodError
	case apiv1.PodSucceeded:
		return ketchv1.PodSucceeded
	default:
		return ketchv1.PodError
	}
}
