package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newPlatformDeleteCmd(cli resourceGetDeleter, logWriter io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "delete PLATFORM",
		Short: "Delete a platform",
		Long:  "Delete a platform that was previously added",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return deletePlatformByName(cmd.Context(), cli, args[0], logWriter)
		},
	}
}

type resourceGetDeleter interface {
	resourceGetter
	resourceDeleter
}

func deletePlatformByName(ctx context.Context, cli resourceGetDeleter, platformName string, w io.Writer) error {
	p, err := platformGet(ctx, cli, platformName)
	if err != nil {
		return fmt.Errorf("unable to find platform %q: %w", platformName, err)
	}
	if err := platformDelete(ctx, cli, p); err != nil {
		return fmt.Errorf("unable to remove platform %q: %w", platformName, err)
	}
	fmt.Fprintf(w, "Successfully removed platform %q\n", platformName)
	return nil
}
