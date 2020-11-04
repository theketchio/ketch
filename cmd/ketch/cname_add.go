package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
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
	cmd.Flags().StringVarP(&options.appName, "app", "a", "", "The name of the app.")
	cmd.MarkFlagRequired("app")
	return cmd
}

type cnameAddOptions struct {
	appName string
	cname   string
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
		if cname == options.cname {
			return nil
		}
	}
	app.Spec.Ingress.Cnames = append(app.Spec.Ingress.Cnames, options.cname)
	if err := cfg.Client().Update(ctx, &app); err != nil {
		return fmt.Errorf("failed to update the app: %w", err)
	}
	return nil
}
