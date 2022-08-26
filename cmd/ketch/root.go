package main

import (
	"io"

	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/theketchio/ketch/cmd/ketch/configuration"
	"github.com/theketchio/ketch/internal/pack"
	"github.com/theketchio/ketch/internal/templates"
)

type config interface {
	Client() client.Client
	Storage() templates.Client
	// KubernetesClient returns kubernetes typed client. It's used to work with standard kubernetes types.
	KubernetesClient() kubernetes.Interface
	// DynamicClient returns kubernetes dynamic client. It's used to work with CRDs for which we don't have go types like ClusterIssuer.
	DynamicClient() dynamic.Interface
}

// RootCmd represents the base command when called without any subcommands
func newRootCmd(cfg config, out io.Writer, packSvc *pack.Client, ketchConfig configuration.KetchConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "ketch",
		Short:         "Manage your applications and your cloud resources",
		Long:          `For details see https://theketch.io`,
		Version:       version,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(newAppCmd(cfg, out, packSvc, ketchConfig.DefaultBuilder))
	cmd.AddCommand(newBuilderCmd(ketchConfig, out))
	cmd.AddCommand(newCnameCmd(cfg, out))
	cmd.AddCommand(newEnvCmd(cfg, out))
	cmd.AddCommand(newJobCmd(cfg, out))
	cmd.AddCommand(newIngressCmd(cfg, out))
	cmd.AddCommand(newCompletionCmd())
	return cmd
}
