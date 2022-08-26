package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/theketchio/ketch/cmd/ketch/output"
	ketchv1 "github.com/theketchio/ketch/internal/api/v1"
	"github.com/theketchio/ketch/internal/utils"
)

type appListOutput struct {
	Name        string `json:"name" yaml:"name"`
	Namespace   string `json:"namespace" yaml:"namespace"`
	State       string `json:"state" yaml:"state"`
	Addresses   string `json:"addresses" yaml:"addresses"`
	Builder     string `json:"builder" yaml:"builder"`
	Description string `json:"description" yaml:"description"`
}

const appListHelp = `
List all apps running on a kubernetes cluster.
`

func newAppListCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all apps.",
		Long:  appListHelp,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return appList(cmd.Context(), cfg, out)
		},
	}
	return cmd
}

func appList(ctx context.Context, cfg config, out io.Writer) error {
	apps := ketchv1.AppList{}
	if err := cfg.Client().List(ctx, &apps); err != nil {
		return fmt.Errorf("failed to list apps: %w", err)
	}
	allPods, err := allAppsPods(ctx, cfg, apps.Items)
	if err != nil {
		return fmt.Errorf("failed to list apps pods: %w", err)
	}
	return output.Write(generateAppListOutput(apps, allPods), out, "column")
}

func generateAppListOutput(apps ketchv1.AppList, allPods *corev1.PodList) []appListOutput {
	var outputs []appListOutput
	for _, item := range apps.Items {
		pods := filterAppPods(item.Name, allPods.Items)
		urls := strings.Join(item.CNames(), " ")
		outputs = append(outputs, appListOutput{
			Name:        item.Name,
			Namespace:   item.Spec.Namespace,
			State:       appState(pods),
			Addresses:   urls,
			Builder:     item.Spec.Builder,
			Description: item.Spec.Description,
		})
	}
	return outputs
}

func allAppsPods(ctx context.Context, cfg config, apps []ketchv1.App) (*corev1.PodList, error) {
	if len(apps) == 0 {
		return &corev1.PodList{}, nil
	}
	selector := &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      utils.KetchAppNameLabel,
				Operator: "Exists",
			},
		},
	}
	s, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, err
	}

	return cfg.KubernetesClient().CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: s.String(),
	})
}

func filterAppPods(appName string, pods []corev1.Pod) []corev1.Pod {
	var appPods []corev1.Pod
	for _, pod := range pods {
		if pod.Labels[utils.KetchAppNameLabel] == appName {
			appPods = append(appPods, pod)
		}
	}
	return appPods
}

func appListNames(cfg config, nameFilter ...string) ([]string, error) {
	apps := ketchv1.AppList{}
	if err := cfg.Client().List(context.TODO(), &apps); err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}

	appNames := make([]string, 0)
	for _, a := range apps.Items {
		if len(nameFilter) == 0 {
			appNames = append(appNames, a.Name)
		}

		for _, filter := range nameFilter {
			if strings.Contains(a.Name, filter) {
				appNames = append(appNames, a.Name)
			}
		}
	}
	return appNames, nil
}
