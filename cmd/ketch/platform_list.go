package main

import (
	"context"
	"fmt"
	"io"

	tw "github.com/olekukonko/tablewriter"
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

func writePlatformList(ctx context.Context, lister resourceLister, logWriter io.Writer) error {
	platforms, err := platformList(ctx, lister)
	if err != nil {
		return fmt.Errorf("could not list platforms: %w", err)
	}
	table := tw.NewWriter(logWriter)
	table.SetHeader([]string{
		"NAME",
		"IMAGE",
		"DESCRIPTION",
	})
	for _, platform := range platforms.Items {
		table.Append([]string{platform.Name, platform.Spec.Image, platform.Spec.Description})
	}
	table.Render()
	return nil
}
