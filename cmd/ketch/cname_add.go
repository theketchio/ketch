package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/deploy"
	"github.com/shipa-corp/ketch/internal/validation"
)

const cnameAddHelp = `
Add a new CNAME to an application.
`

func newCnameAddCmd(cfg config, out io.Writer) *cobra.Command {
	options := cnameAddOptions{}
	cmd := &cobra.Command{
		Use:   "add CNAME",
		Args:  cobra.ExactValidArgs(1),
		Short: "Add a new CNAME to an application.",
		Long:  cnameAddHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.cname = args[0]
			return cnameAdd(cmd.Context(), cfg, options, out)
		},
	}
	cmd.Flags().StringVarP(&options.appName, deploy.FlagApp, deploy.FlagAppShort, "", "The name of the app.")
	cmd.MarkFlagRequired("app")
	cmd.Flags().BoolVar(&options.secure, "secure", false, "Whether the CName should be https")

	cmd.RegisterFlagCompletionFunc(deploy.FlagApp, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return autoCompleteAppNames(cfg, toComplete)
	})
	return cmd
}

type cnameAddOptions struct {
	appName string
	cname   string
	secure  bool
}

func cnameAdd(ctx context.Context, cfg config, options cnameAddOptions, out io.Writer) error {
	if err := validation.ValidateCname(options.cname); err != nil {
		return err
	}
	app := ketchv1.App{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.appName}, &app); err != nil {
		return fmt.Errorf("failed to get the app: %w", err)
	}
	for _, cname := range app.Spec.Ingress.Cnames {
		if cname.Name == options.cname {
			return nil
		}
	}
	var framework ketchv1.Framework
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: app.Spec.Framework}, &framework); err != nil {
		return fmt.Errorf("failed to get the framework: %w", err)
	}
	if options.secure && len(framework.Spec.IngressController.ClusterIssuer) == 0 {
		return ErrClusterIssuerRequired
	}
	app.Spec.Ingress.Cnames = append(app.Spec.Ingress.Cnames, ketchv1.Cname{Name: options.cname, Secure: options.secure})
	if err := cfg.Client().Update(ctx, &app); err != nil {
		return fmt.Errorf("failed to update the app: %w", err)
	}
	return nil
}
