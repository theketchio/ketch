package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/deploy"
)

const cnameRemoveHelp = `
Remove a CNAME from an application.
`

func newCnameRemoveCmd(cfg config, out io.Writer) *cobra.Command {
	options := cnameRemoveOptions{}
	cmd := &cobra.Command{
		Use:   "remove CNAME",
		Args:  cobra.ExactValidArgs(1),
		Short: "Remove a CNAME from an application.",
		Long:  cnameRemoveHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.cname = args[0]
			return cnameRemove(cmd.Context(), cfg, options, out)
		},
	}
	cmd.Flags().StringVarP(&options.appName, deploy.FlagApp, deploy.FlagAppShort, "", "The name of the app.")
	cmd.MarkFlagRequired(deploy.FlagApp)
	cmd.RegisterFlagCompletionFunc(deploy.FlagApp, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return autoCompleteAppNames(cfg, toComplete)
	})
	return cmd
}

type cnameRemoveOptions struct {
	appName string
	cname   string
}

func cnameRemove(ctx context.Context, cfg config, options cnameRemoveOptions, out io.Writer) error {
	app := ketchv1.App{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.appName}, &app); err != nil {
		return fmt.Errorf("failed to get the app: %w", err)
	}
	cnames := make([]string, 0, len(app.Spec.Ingress.Cnames))
	for _, cname := range app.Spec.Ingress.Cnames {
		if cname == options.cname {
			continue
		}
		cnames = append(cnames, cname)
	}
	app.Spec.Ingress.Cnames = cnames
	if err := cfg.Client().Update(ctx, &app); err != nil {
		return fmt.Errorf("failed to update the app: %w", err)
	}
	return nil
}
