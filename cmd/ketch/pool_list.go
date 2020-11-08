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

const poolListHelp = `
List all pools available for deploy.
`

func newPoolListCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all pools available for deploy.",
		Long:  poolListHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			return poolList(cmd.Context(), cfg, out)
		},
	}
	return cmd
}

func poolList(ctx context.Context, cfg config, out io.Writer) error {
	pools := ketchv1.PoolList{}
	if err := cfg.Client().List(ctx, &pools); err != nil {
		return fmt.Errorf("failed to get list of pools: %w", err)
	}

	w := tabwriter.NewWriter(out, 0, 4, 4, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tNAMESPACE\tINGRESS TYPE\tINGRESS CLASS NAME\tCLUSTER ISSUER\tAPPS")

	for _, item := range pools.Items {
		apps := fmt.Sprintf("%d", len(item.Status.Apps))
		if item.Spec.AppQuotaLimit > 0 {
			apps = fmt.Sprintf("%d/%d", len(item.Status.Apps), item.Spec.AppQuotaLimit)
		}
		line := []string{
			item.Name,
			string(item.Status.Phase),
			item.Spec.NamespaceName,
			item.Spec.IngressController.IngressType.String(),
			item.Spec.IngressController.ClassName,
			item.Spec.IngressController.ClusterIssuer,
			apps}
		fmt.Fprintln(w, strings.Join(line, "\t"))
	}
	w.Flush()
	return nil
}
