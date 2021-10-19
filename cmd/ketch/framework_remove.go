package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

const (
	frameworkRemoveHelp = `
Remove an existing framework.
`
	skipNsRemovalMsg = "Skipping namespace removal..."
)

func newFrameworkRemoveCmd(cfg config, out io.Writer) *cobra.Command {
	options := frameworkRemoveOptions{}
	cmd := &cobra.Command{
		Use:   "remove FRAMEWORK",
		Short: "Remove an existing framework.",
		Long:  frameworkRemoveHelp,
		Args:  cobra.ExactValidArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Name = args[0]
			return frameworkRemove(cmd.Context(), cfg, options, out)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return autoCompleteFrameworkNames(cfg, toComplete)
		},
	}
	return cmd
}

type frameworkRemoveOptions struct {
	Name string
}

func frameworkRemove(ctx context.Context, cfg config, options frameworkRemoveOptions, out io.Writer) error {
	var framework ketchv1.Framework

	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.Name}, &framework); err != nil {
		return fmt.Errorf("failed to get framework: %w", err)
	}

	if err := pruneRemovedAppsFromStatus(ctx, cfg, framework); err != nil {
		return fmt.Errorf("failed to update framework's apps: %w", err)
	}
	if userWantsToRemoveNamespace(framework.Spec.NamespaceName, out) {
		if err := checkNamespaceAdditionalFrameworks(ctx, cfg, &framework); err != nil {
			printNsRemovalErr(out, err)
		} else {
			if err := removeNamespace(ctx, cfg, &framework); err != nil {
				printNsRemovalErr(out, err)
			} else {
				fmt.Fprintln(out, "Namespace successfully removed!")
			}
		}
	}

	if err := cfg.Client().Delete(ctx, &framework); err != nil {
		return fmt.Errorf("failed to remove the framework: %w", err)
	}

	fmt.Fprintln(out, "Framework successfully removed!")

	return nil
}

func userWantsToRemoveNamespace(ns string, out io.Writer) bool {
	response := promptToRemoveNamespace(ns, out)
	return handleNamespaceRemovalResponse(response, ns, out)
}

func promptToRemoveNamespace(ns string, out io.Writer) string {
	fmt.Fprintf(out, "Do you want to remove the namespace along with the framework? Please enter namespace to confirm (%s): ", ns)

	var response string
	fmt.Scanln(&response)

	return response
}

func handleNamespaceRemovalResponse(response, ns string, out io.Writer) bool {
	if response != ns {
		fmt.Fprintln(out, skipNsRemovalMsg)
		return false
	}

	return true
}

func checkNamespaceAdditionalFrameworks(ctx context.Context, cfg config, targetFramework *ketchv1.Framework) error {
	var frameworks ketchv1.FrameworkList

	if err := cfg.Client().List(ctx, &frameworks); err != nil {
		return fmt.Errorf("failed to list frameworks: %w", err)
	}

	for _, p := range frameworks.Items {
		if p.Name != targetFramework.Name && p.Spec.NamespaceName == targetFramework.Spec.NamespaceName {
			return fmt.Errorf(
				"Namespace contains other frameworks than %s, and cannot be removed:\nFrameworks in target namespace:%+v",
				targetFramework.Name, frameworks.Items)
		}
	}

	return nil
}

func printNsRemovalErr(out io.Writer, err error) {
	fmt.Fprintf(out, "%s\n%s", err, skipNsRemovalMsg)
}

func removeNamespace(ctx context.Context, cfg config, framework *ketchv1.Framework) error {
	var ns corev1.Namespace

	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: framework.Spec.NamespaceName}, &ns); err != nil {
		return fmt.Errorf("failed to get namespace: %w", err)
	}

	if err := cfg.Client().Delete(ctx, &ns); err != nil {
		return fmt.Errorf("failed to remove the namespace: %w", err)
	}

	return nil
}

// pruneRemovedAppsFromStatus lists apps and removes any apps that are not present on the system from a framework's status.
func pruneRemovedAppsFromStatus(ctx context.Context, cfg config, framework ketchv1.Framework) error {
	var apps ketchv1.AppList
	if err := cfg.Client().List(ctx, &apps); err != nil {
		return fmt.Errorf("failed to list framework apps: %w", err)
	}
	var updatedApps []string
	for _, frameworkApp := range framework.Status.Apps {
		for _, app := range apps.Items {
			if app.Name == frameworkApp {
				updatedApps = append(updatedApps, frameworkApp)
			}
		}
	}
	patchedFramework := framework
	patchedFramework.Status.Apps = updatedApps
	patch := client.MergeFrom(&framework)
	return cfg.Client().Status().Patch(ctx, &patchedFramework, patch)
}
