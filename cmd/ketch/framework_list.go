package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/pflag"

	"github.com/spf13/cobra"

	"github.com/shipa-corp/ketch/cmd/ketch/output"
	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

const frameworkListHelp = `
List all frameworks available for deploy.
`

type frameworkListOutput struct {
	Name             string `json:"name" yaml:"name"`
	Status           string `json:"status" yaml:"status"`
	Namespace        string `json:"namespace" yaml:"namespace"`
	IngressType      string `json:"ingressType" yaml:"ingressType"`
	IngressClassName string `json:"ingressClassName" yaml:"ingressClassName"`
	ClusterIssuer    string `json:"clusterIssuer" yaml:"clusterIssuer"`
	Apps             string `json:"apps" yaml:"apps"`
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
	cmd.Flags().StringP("output", "o", "", "used to specify output, e.g. --output format=json")
	return cmd
}

func frameworkList(ctx context.Context, cfg config, out io.Writer, flags *pflag.FlagSet) error {
	frameworks := ketchv1.FrameworkList{}
	if err := cfg.Client().List(ctx, &frameworks); err != nil {
		return fmt.Errorf("failed to get list of frameworks: %w", err)
	}
	outputFlag, _ := flags.GetString("output")
	return output.Write(generateFrameworkListOutput(frameworks), out, outputFlag)
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
