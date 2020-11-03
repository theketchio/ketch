package main

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

const unitRemoveHelp = `
Remove units from a process of an application.
`

func newUnitRemoveCmd(cfg config, out io.Writer) *cobra.Command {
	options := unitRemoveOptions{}
	cmd := &cobra.Command{
		Use:   "remove #UNITS",
		Args:  cobra.ExactValidArgs(1),
		Short: "Remove units from a process of an application.",
		Long:  unitRemoveHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			quantity, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			options.quantity = quantity
			return unitRemove(cmd.Context(), cfg, options, out)
		},
	}
	cmd.Flags().StringVarP(&options.appName, "app", "a", "", "The name of the app.")
	cmd.Flags().StringVarP(&options.processName, "process", "p", "", "Process name.")
	cmd.Flags().IntVarP(&options.deploymentVersion, "version", "v", 0, "Deployment version.")
	cmd.MarkFlagRequired("app")
	return cmd
}

type unitRemoveOptions struct {
	appName           string
	processName       string
	deploymentVersion int

	quantity int
}

func unitRemove(ctx context.Context, cfg config, options unitRemoveOptions, out io.Writer) error {
	app := ketchv1.App{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.appName}, &app); err != nil {
		return fmt.Errorf("failed to get the app: %w", err)
	}
	s := ketchv1.NewSelector(options.deploymentVersion, options.processName)
	if err := app.AddUnits(s, -options.quantity); err != nil {
		return fmt.Errorf("failed to update the app: %w", err)
	}
	if err := cfg.Client().Update(ctx, &app); err != nil {
		return fmt.Errorf("failed to update the app: %w", err)
	}
	return nil
}
