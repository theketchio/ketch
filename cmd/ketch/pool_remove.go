package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

const poolRemoveHelp = `
Remove an existing pool.
`

func newPoolRemoveCmd(cfg config, out io.Writer) *cobra.Command {
	options := poolRemoveOptions{}
	cmd := &cobra.Command{
		Use:   "remove POOL",
		Short: "Remove an existing pool.",
		Long:  poolRemoveHelp,
		Args:  cobra.ExactValidArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Name = args[0]
			return poolRemove(cmd.Context(), cfg, options, out)
		},
	}
	return cmd
}

type poolRemoveOptions struct {
	Name string
}

func poolRemove(ctx context.Context, cfg config, options poolRemoveOptions, out io.Writer) error {
	var pool ketchv1.Pool
	var ns corev1.Namespace

	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.Name}, &pool); err != nil {
		return fmt.Errorf("failed to get pool: %w", err)
	}

	// Get namespace that was created with the pool so it can be removed
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: pool.Spec.NamespaceName}, &ns); err != nil {
		return fmt.Errorf("failed to get namespace: %w", err)
	}

	if err := cfg.Client().Delete(ctx, &pool); err != nil {
		return fmt.Errorf("failed to remove the pool: %w", err)
	}

	if userWantsToRemoveNamespace(ns.Name, out) {
		if err := cfg.Client().Delete(ctx, &ns); err != nil {
			return fmt.Errorf("failed to remove the namespace: %w", err)
		}
		fmt.Fprintln(out, "Namespace successfully removed!")
	}

	fmt.Fprintln(out, "Pool successfully removed!")

	return nil
}

func userWantsToRemoveNamespace(ns string, out io.Writer) bool {
	response := promptToRemoveNamespace(ns, out)
	return handleNamespaceRemovalResponse(response, ns, out)
}

func promptToRemoveNamespace(ns string, out io.Writer) string {
	fmt.Fprintf(out, "Do you want to remove the namespace along with the pool? Please enter namespace to confirm (%s): ", ns)

	var response string
	fmt.Scanln(&response)

	return response
}

func handleNamespaceRemovalResponse(response, ns string, out io.Writer) bool {
	if response != ns {
		fmt.Fprintln(out, "Skipping namespace removal...")
		return false
	}

	return true
}
