package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/thediveo/enumflag"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

const frameworkUpdateHelp = `
Update a framework.
`

func newFrameworkUpdateCmd(cfg config, out io.Writer) *cobra.Command {
	options := frameworkUpdateOptions{}
	cmd := &cobra.Command{
		Use:   "update POOL",
		Args:  cobra.ExactValidArgs(1),
		Short: "Update a framework.",
		Long:  frameworkUpdateHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.name = args[0]
			options.appQuotaLimitSet = cmd.Flags().Changed("app-quota-limit")
			options.namespaceSet = cmd.Flags().Changed("namespace")
			options.ingressClassNameSet = cmd.Flags().Changed("ingress-class-name")
			options.ingressServiceEndpointSet = cmd.Flags().Changed("ingress-service-endpoint")
			options.ingressTypeSet = cmd.Flags().Changed("ingress-type")
			options.ingressClusterIssuerSet = cmd.Flags().Changed("cluster-issuer")
			return frameworkUpdate(cmd.Context(), cfg, options, out)
		},
	}
	cmd.Flags().StringVar(&options.namespace, "namespace", "", "Kubernetes namespace for this framework")
	cmd.Flags().IntVar(&options.appQuotaLimit, "app-quota-limit", 0, "Quota limit for app when adding it to this framework")
	cmd.Flags().StringVar(&options.ingressClassName, "ingress-class-name", "", "if set, it is used as kubernetes.io/ingress.class annotations")
	cmd.Flags().StringVar(&options.ingressServiceEndpoint, "ingress-service-endpoint", "", "an IP address or dns name of the ingress controller's Service")
	cmd.Flags().StringVar(&options.ingressClusterIssuer, "cluster-issuer", "", "ClusterIssuer to obtain SSL certificates")
	cmd.Flags().Var(enumflag.New(&options.ingressType, "ingress-type", ingressTypeIds, enumflag.EnumCaseInsensitive), "ingress-type", "ingress controller type: traefik or istio")
	return cmd
}

type frameworkUpdateOptions struct {
	name string

	appQuotaLimitSet          bool
	appQuotaLimit             int
	namespaceSet              bool
	namespace                 string
	ingressClassNameSet       bool
	ingressClassName          string
	ingressClusterIssuerSet   bool
	ingressClusterIssuer      string
	ingressServiceEndpointSet bool
	ingressServiceEndpoint    string
	ingressTypeSet            bool
	ingressType               ingressType
}

func frameworkUpdate(ctx context.Context, cfg config, options frameworkUpdateOptions, out io.Writer) error {
	framework := ketchv1.Framework{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.name}, &framework); err != nil {
		return fmt.Errorf("failed to get the framework: %w", err)
	}
	if options.appQuotaLimitSet {
		framework.Spec.AppQuotaLimit = options.appQuotaLimit
	}
	if options.namespaceSet {
		framework.Spec.NamespaceName = options.namespace
	}
	if options.ingressClassNameSet {
		framework.Spec.IngressController.ClassName = options.ingressClassName
	}
	if options.ingressServiceEndpointSet {
		framework.Spec.IngressController.ServiceEndpoint = options.ingressServiceEndpoint
	}
	if options.ingressTypeSet {
		framework.Spec.IngressController.IngressType = options.ingressType.ingressControllerType()
	}
	if options.ingressClusterIssuerSet {
		exists, err := clusterIssuerExist(cfg.DynamicClient(), ctx, options.ingressClusterIssuer)
		if err != nil {
			return err
		}
		if !exists {
			return ErrClusterIssuerNotFound
		}
		framework.Spec.IngressController.ClusterIssuer = options.ingressClusterIssuer
	}
	if err := cfg.Client().Update(ctx, &framework); err != nil {
		return fmt.Errorf("failed to update the framework: %w", err)
	}
	fmt.Fprintln(out, "Successfully updated!")
	return nil
}
