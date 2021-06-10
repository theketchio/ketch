package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"

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
	IngresType       string
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

	outputs := generateFrameworkListOutput(frameworks)
	outputFlag, err := flags.GetString("output")
	if err != nil {
		outputFlag = ""
	}
	switch outputFlag {
	case "json", "JSON":
		j, err := json.MarshalIndent(outputs, "", "\t")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(j))
	case "yaml", "YAML":
		y, err := yaml.Marshal(outputs)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(y))
	default:
		w := tabwriter.NewWriter(out, 0, 4, 4, ' ', 0)
		fmt.Fprintln(w, "NAME\tSTATUS\tNAMESPACE\tINGRESS TYPE\tINGRESS CLASS NAME\tCLUSTER ISSUER\tAPPS")
		for _, output := range outputs {
			line := []string{output.Name, output.Status, output.Namespace, output.IngresType, output.IngressClassName, output.ClusterIssuer, output.Apps}
			fmt.Fprintln(w, strings.Join(line, "\t"))
		}
		w.Flush()
	}
	return nil
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
			IngresType:       item.Spec.IngressController.IngressType.String(),
			IngressClassName: item.Spec.IngressController.ClassName,
			ClusterIssuer:    item.Spec.IngressController.ClusterIssuer,
			Apps:             apps,
		})
	}
	return output
}
