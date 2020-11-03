package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

const poolUpdateHelp = `
Update a pool.
`

func newPoolUpdateCmd(cfg config, out io.Writer) *cobra.Command {
	options := poolUpdateOptions{}
	cmd := &cobra.Command{
		Use:   "update POOL",
		Args:  cobra.ExactValidArgs(1),
		Short: "Update a pool.",
		Long:  poolUpdateHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.name = args[0]
			options.appQuotaLimitSet = cmd.Flags().Changed("app-quota-limit")
			options.kubeNamespaceSet = cmd.Flags().Changed("kube-namespace")
			options.ingressClassNameSet = cmd.Flags().Changed("ingress-class-name")
			options.ingressDomainNameSet = cmd.Flags().Changed("ingress-domain")
			options.ingressServiceEndpointSet = cmd.Flags().Changed("ingress-service-endpoint")
			return poolUpdate(cmd.Context(), cfg, options, out)
		},
	}
	cmd.Flags().StringVar(&options.kubeNamespace, "kube-namespace", "", "Kubernetes namespace for this pool")
	cmd.Flags().IntVar(&options.appQuotaLimit, "app-quota-limit", 0, "Quota limit for app when adding it to this pool")
	cmd.Flags().StringVar(&options.ingressClassName, "ingress-class-name", "", "if set, it is used as kubernetes.io/ingress.class annotations")
	cmd.Flags().StringVar(&options.ingressDomainName, "ingress-domain", "shipa.cloud", "domain name for the default URL")
	cmd.Flags().StringVar(&options.ingressServiceEndpoint, "ingress-service-endpoint", "", "an IP address or dns name of the ingress controller's Service")
	return cmd
}

type poolUpdateOptions struct {
	name string

	appQuotaLimitSet          bool
	appQuotaLimit             int
	kubeNamespaceSet          bool
	kubeNamespace             string
	ingressClassNameSet       bool
	ingressClassName          string
	ingressDomainNameSet      bool
	ingressDomainName         string
	ingressServiceEndpointSet bool
	ingressServiceEndpoint    string
}

func poolUpdate(ctx context.Context, cfg config, options poolUpdateOptions, out io.Writer) error {
	pool := ketchv1.Pool{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.name}, &pool); err != nil {
		return fmt.Errorf("failed to get the pool: %w", err)
	}
	if options.appQuotaLimitSet {
		pool.Spec.AppQuotaLimit = options.appQuotaLimit
	}
	if options.kubeNamespaceSet {
		pool.Spec.NamespaceName = options.kubeNamespace
	}
	if options.ingressClassNameSet {
		pool.Spec.IngressController.ClassName = options.ingressClassName
	}
	if options.ingressDomainNameSet {
		pool.Spec.IngressController.Domain = options.ingressDomainName
	}
	if options.ingressServiceEndpointSet {
		pool.Spec.IngressController.ServiceEndpoint = options.ingressServiceEndpoint
	}
	if err := cfg.Client().Update(ctx, &pool); err != nil {
		return fmt.Errorf("failed to update the pool: %w", err)
	}
	return nil
}
