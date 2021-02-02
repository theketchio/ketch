package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/thediveo/enumflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/validation"
)

const poolAddHelp = `
Add a new pool.
`

type ingressType enumflag.Flag

const (
	traefik ingressType = iota
	istio
)

const (
	defaultIstioIngressClassName   = "istio"
	defaultTraefikIngressClassName = "traefik"
)

var ingressTypeIds = map[ingressType][]string{
	traefik: {ketchv1.TraefikIngressControllerType.String()},
	istio:   {ketchv1.IstioIngressControllerType.String()},
}

type addPoolFn func(ctx context.Context, cfg config, options poolAddOptions, out io.Writer) error

func newPoolAddCmd(cfg config, out io.Writer, addPool addPoolFn) *cobra.Command {
	options := poolAddOptions{}
	cmd := &cobra.Command{
		Use:   "add POOL",
		Args:  cobra.ExactValidArgs(1),
		Short: "Add a new pool.",
		Long:  poolAddHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.name = args[0]
			options.ingressClassNameSet = cmd.Flags().Changed("ingress-class-name")
			return addPool(cmd.Context(), cfg, options, out)
		},
	}
	cmd.Flags().StringVar(&options.namespace, "namespace", "", "Kubernetes namespace for this pool")
	cmd.Flags().IntVar(&options.appQuotaLimit, "app-quota-limit", -1, "Quota limit for app when adding it to this pool")
	cmd.Flags().StringVar(&options.ingressClassName, "ingress-class-name", "", `if set, it is used as kubernetes.io/ingress.class annotations. Ketch uses "istio" class name for istio ingress controller, if class name is not specified`)
	cmd.Flags().StringVar(&options.ingressClusterIssuer, "cluster-issuer", "", "ClusterIssuer to obtain SSL certificates")
	cmd.Flags().StringVar(&options.ingressServiceEndpoint, "ingress-service-endpoint", "", "an IP address or dns name of the ingress controller's Service")
	cmd.Flags().Var(enumflag.New(&options.ingressType, "ingress-type", ingressTypeIds, enumflag.EnumCaseInsensitive), "ingress-type", "ingress controller type: traefik or istio")
	return cmd
}

type poolAddOptions struct {
	name string

	appQuotaLimit int
	namespace     string

	ingressClassNameSet    bool
	ingressClassName       string
	ingressClusterIssuer   string
	ingressServiceEndpoint string
	ingressType            ingressType
}

func addPool(ctx context.Context, cfg config, options poolAddOptions, out io.Writer) error {
	if !validation.ValidateName(options.name) {
		return ErrInvalidPoolName
	}
	namespace := fmt.Sprintf("ketch-%s", options.name)
	if len(options.namespace) > 0 {
		namespace = options.namespace
	}
	if len(options.ingressClusterIssuer) > 0 {
		exists, err := clusterIssuerExist(cfg.DynamicClient(), ctx, options.ingressClusterIssuer)
		if err != nil {
			return err
		}
		if !exists {
			return ErrClusterIssuerNotFound
		}
	}
	pool := ketchv1.Pool{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: options.name,
		},
		Spec: ketchv1.PoolSpec{
			NamespaceName: namespace,
			AppQuotaLimit: options.appQuotaLimit,
			IngressController: ketchv1.IngressControllerSpec{
				ClassName:       options.IngressClassName(),
				ServiceEndpoint: options.ingressServiceEndpoint,
				ClusterIssuer:   options.ingressClusterIssuer,
				IngressType:     options.ingressType.ingressControllerType(),
			},
		},
		Status: ketchv1.PoolStatus{},
	}
	if err := cfg.Client().Create(ctx, &pool); err != nil {
		return fmt.Errorf("failed to create pool: %w", err)
	}
	fmt.Fprintln(out, "Successfully added!")
	return nil
}

func (o poolAddOptions) IngressClassName() string {
	if !o.ingressClassNameSet && o.ingressType.ingressControllerType() == ketchv1.IstioIngressControllerType {
		return defaultIstioIngressClassName
	}

	if !o.ingressClassNameSet && o.ingressType.ingressControllerType() == ketchv1.TraefikIngressControllerType {
		return defaultTraefikIngressClassName
	}

	return o.ingressClassName
}

func (t ingressType) ingressControllerType() ketchv1.IngressControllerType {
	switch t {
	case istio:
		return ketchv1.IstioIngressControllerType
	default:
		return ketchv1.TraefikIngressControllerType
	}
}
