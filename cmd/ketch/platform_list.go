package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newPlatformListCmd(lister resourceLister, logWriter io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List platforms",
		Long:  "Displays list of added platforms",
		RunE: func(cmd *cobra.Command, args []string) error {
			return writePlatformList(cmd.Context(), lister, logWriter)
		},
	}
}

func writePlatformList(ctx context.Context, lister resourceLister, out io.Writer) error {
	platforms, err := platformList(ctx, lister)
	if err != nil {
		return fmt.Errorf("could not list platforms: %w", err)
	}
	tw := tabwriter.NewWriter(out, 0, 4, 4, ' ', 0)
	fmt.Fprintln(tw, "NAME\tIMAGE\tDESCRIPTION")
	for _, platform := range platforms.Items {
		line := []string{
			platform.Name,
			platform.Spec.Image,
			platform.Spec.Description,
		}
		fmt.Fprintln(tw, strings.Join(line, "\t"))
	}
	tw.Flush()
	return nil
}
