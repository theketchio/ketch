package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

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
	pools := ketchv1.PoolList{}
	if err := cfg.Client().List(ctx, &pools); err != nil {
		return fmt.Errorf("failed to list pools: %w", err)
	}
	poolsByName := make(map[string]ketchv1.Pool, len(pools.Items))
	for _, pool := range pools.Items {
		poolsByName[pool.Name] = pool
	}
	w := tabwriter.NewWriter(out, 0, 4, 4, ' ', 0)
	fmt.Fprintln(w, "NAME\tPOOL\tSTATUS\tUNITS\tADDRESSES\tDESCRIPTION")
	for _, item := range apps.Items {
		pool := poolsByName[item.Spec.Pool]
		urls := strings.Join(item.URLs(&pool), " ")
		line := []string{item.Name, item.Spec.Pool, string(item.Status.Phase), fmt.Sprintf("%d", item.Units()), urls, item.Spec.Description}
		fmt.Fprintln(w, strings.Join(line, "\t"))
	}
	w.Flush()
	return nil
}
