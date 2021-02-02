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
			options.namespaceSet = cmd.Flags().Changed("namespace")
			options.ingressClassNameSet = cmd.Flags().Changed("ingress-class-name")
			options.ingressServiceEndpointSet = cmd.Flags().Changed("ingress-service-endpoint")
			options.ingressTypeSet = cmd.Flags().Changed("ingress-type")
			options.ingressClusterIssuerSet = cmd.Flags().Changed("cluster-issuer")
			return poolUpdate(cmd.Context(), cfg, options, out)
		},
	}
	cmd.Flags().StringVar(&options.namespace, "namespace", "", "Kubernetes namespace for this pool")
	cmd.Flags().IntVar(&options.appQuotaLimit, "app-quota-limit", 0, "Quota limit for app when adding it to this pool")
	cmd.Flags().StringVar(&options.ingressClassName, "ingress-class-name", "", "if set, it is used as kubernetes.io/ingress.class annotations")
	cmd.Flags().StringVar(&options.ingressServiceEndpoint, "ingress-service-endpoint", "", "an IP address or dns name of the ingress controller's Service")
	cmd.Flags().StringVar(&options.ingressClusterIssuer, "cluster-issuer", "", "ClusterIssuer to obtain SSL certificates")
	cmd.Flags().Var(enumflag.New(&options.ingressType, "ingress-type", ingressTypeIds, enumflag.EnumCaseInsensitive), "ingress-type", "ingress controller type: traefik or istio")
	return cmd
}

type poolUpdateOptions struct {
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

func poolUpdate(ctx context.Context, cfg config, options poolUpdateOptions, out io.Writer) error {
	pool := ketchv1.Pool{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.name}, &pool); err != nil {
		return fmt.Errorf("failed to get the pool: %w", err)
	}
	if options.appQuotaLimitSet {
		pool.Spec.AppQuotaLimit = options.appQuotaLimit
	}
	if options.namespaceSet {
		pool.Spec.NamespaceName = options.namespace
	}
	if options.ingressClassNameSet {
		pool.Spec.IngressController.ClassName = options.ingressClassName
	}
	if options.ingressServiceEndpointSet {
		pool.Spec.IngressController.ServiceEndpoint = options.ingressServiceEndpoint
	}
	if options.ingressTypeSet {
		pool.Spec.IngressController.IngressType = options.ingressType.ingressControllerType()
	}
	if options.ingressClusterIssuerSet {
		exists, err := clusterIssuerExist(cfg.DynamicClient(), ctx, options.ingressClusterIssuer)
		if err != nil {
			return err
		}
		if !exists {
			return ErrClusterIssuerNotFound
		}
		pool.Spec.IngressController.ClusterIssuer = options.ingressClusterIssuer
	}
	if err := cfg.Client().Update(ctx, &pool); err != nil {
		return fmt.Errorf("failed to update the pool: %w", err)
	}
	fmt.Fprintln(out, "Successfully updated!")
	return nil
}
