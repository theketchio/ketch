package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/pflag"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/shipa-corp/ketch/cmd/ketch/output"
	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/utils"
)

type appListOutput struct {
	Name        string `json:"name" yaml:"name"`
	Framework   string `json:"framework" yaml:"framework"`
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
			return appList(cmd.Context(), cfg, out, cmd.Flags())
		},
	}
	cmd.Flags().StringP("output", "o", "", "used to specify output, e.g. --output format=json")
	return cmd
}

func appList(ctx context.Context, cfg config, out io.Writer, flags *pflag.FlagSet) error {
	apps := ketchv1.AppList{}
	if err := cfg.Client().List(ctx, &apps); err != nil {
		return fmt.Errorf("failed to list apps: %w", err)
	}
	frameworks := ketchv1.FrameworkList{}
	if err := cfg.Client().List(ctx, &frameworks); err != nil {
		return fmt.Errorf("failed to list frameworks: %w", err)
	}
	frameworksByName := make(map[string]ketchv1.Framework, len(frameworks.Items))
	for _, framework := range frameworks.Items {
		frameworksByName[framework.Name] = framework
	}
	allPods, err := allAppsPods(ctx, cfg, apps.Items)
	if err != nil {
		return fmt.Errorf("failed to list apps pods: %w", err)
	}
	outputFlag, _ := flags.GetString("output")
	return output.Write(generateAppListOutput(apps, allPods, frameworksByName), out, outputFlag)
}

func generateAppListOutput(apps ketchv1.AppList, allPods *corev1.PodList, frameworksByName map[string]ketchv1.Framework) []appListOutput {
	var outputs []appListOutput
	for _, item := range apps.Items {
		pods := filterAppPods(item.Name, allPods.Items)
		framework := frameworksByName[item.Spec.Framework]
		urls := strings.Join(item.CNames(&framework), " ")
		outputs = append(outputs, appListOutput{
			Name:        item.Name,
			Framework:   item.Spec.Framework,
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
