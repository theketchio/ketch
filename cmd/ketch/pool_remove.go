package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	pool := ketchv1.Pool{
		ObjectMeta: metav1.ObjectMeta{
			Name: options.Name,
		},
	}
	if err := cfg.Client().Delete(ctx, &pool); err != nil {
		return fmt.Errorf("failed to remove the pool: %w", err)
	}
	fmt.Fprintln(out, "Successfully removed!")
	return nil
}
