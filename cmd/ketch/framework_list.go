package main

import (
	"context"
	"fmt"
	"io"

	"github.com/shipa-corp/ketch/cmd/ketch/output"

	"github.com/spf13/pflag"

	"github.com/spf13/cobra"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

const frameworkListHelp = `
List all frameworks available for deploy.
`

type frameworkListOutput struct {
	Name             string
	Status           string
	Namespace        string
	IngressType      string
	IngressClassName string
	ClusterIssuer    string
	Apps             string
}

func newFrameworkListCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all frameworks available for deploy.",
		Long:  frameworkListHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			return frameworkList(cmd.Context(), cfg, out, cmd.Flags())
		},
	}
	return cmd
}

func frameworkList(ctx context.Context, cfg config, out io.Writer, flags *pflag.FlagSet) error {
	frameworks := ketchv1.FrameworkList{}
	if err := cfg.Client().List(ctx, &frameworks); err != nil {
		return fmt.Errorf("failed to get list of frameworks: %w", err)
	}

	return output.Write(generateFrameworkListOutput(frameworks), out, flags)
}

func generateFrameworkListOutput(frameworks ketchv1.FrameworkList) []frameworkListOutput {
	var output []frameworkListOutput
	for _, item := range frameworks.Items {
		apps := fmt.Sprintf("%d", len(item.Status.Apps))
		if item.Spec.AppQuotaLimit > 0 {
			apps = fmt.Sprintf("%d/%d", len(item.Status.Apps), item.Spec.AppQuotaLimit)
		}
		output = append(output, frameworkListOutput{
			Name:             item.Name,
			Status:           string(item.Status.Phase),
			Namespace:        item.Spec.NamespaceName,
			IngressType:      item.Spec.IngressController.IngressType.String(),
			IngressClassName: item.Spec.IngressController.ClassName,
			ClusterIssuer:    item.Spec.IngressController.ClusterIssuer,
			Apps:             apps,
		})
	}
	return output
}
