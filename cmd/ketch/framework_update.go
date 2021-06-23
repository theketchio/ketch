package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"k8s.io/apimachinery/pkg/types"

	"github.com/shipa-corp/ketch/internal/validation"
	"sigs.k8s.io/kustomize/kyaml/yaml"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/spf13/cobra"
	"github.com/thediveo/enumflag"
)

const frameworkUpdateHelp = `
Update a framework.
`

func newFrameworkUpdateCmd(cfg config, out io.Writer) *cobra.Command {
	options := frameworkUpdateOptions{}
	cmd := &cobra.Command{
		Use:   "update FRAMEWORK",
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
			hasFlags := func() bool {
				return cmd.Flags().NFlag() > 0
			}
			return frameworkUpdate(cmd.Context(), cfg, options, out, hasFlags)
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

func frameworkUpdate(ctx context.Context, cfg config, options frameworkUpdateOptions, out io.Writer, hasFlags func() bool) error {
	var framework *ketchv1.Framework
	var err error
	switch {
	case validation.ValidateYamlFilename(options.name):
		if hasFlags() {
			return errors.New("command line flags are not permitted when passing a framework yaml file")
		}
		framework, err = updateFrameworkFromYaml(ctx, cfg, options)
		if err != nil {
			return err
		}
	case validation.ValidateName(options.name):
		framework, err = updateFrameworkFromArgs(ctx, cfg, options)
		if err != nil {
			return err
		}
	default:
		return ErrInvalidFrameworkName
	}

	if len(framework.Spec.IngressController.ClusterIssuer) > 0 {
		exists, err := clusterIssuerExist(cfg.DynamicClient(), ctx, framework.Spec.IngressController.ClusterIssuer)
		if err != nil {
			return err
		}
		if !exists {
			return ErrClusterIssuerNotFound
		}
	}

	if err := cfg.Client().Update(ctx, framework); err != nil {
		return fmt.Errorf("failed to update the framework: %w", err)
	}
	fmt.Fprintln(out, "Successfully updated!")
	return nil
}

// updateFrameworkFromYaml imports a FrameworkSpec definition from a yaml file named in options.name.
// It asserts that the FrameworkSpec has a name and assigns a ketch-prefixed namespaceName if one is not specified.
// It fetches the named Framework, assigns the FrameworkSpec, and returns the Framework.
func updateFrameworkFromYaml(ctx context.Context, cfg config, options frameworkUpdateOptions) (*ketchv1.Framework, error) {
	var spec ketchv1.FrameworkSpec
	f, err := os.Open(options.name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	err = yaml.NewDecoder(f).Decode(&spec)
	if err != nil {
		return nil, err
	}
	if len(spec.Name) == 0 {
		return nil, errors.New("a framework name is required")
	}

	var framework ketchv1.Framework
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: spec.Name}, &framework); err != nil {
		return nil, fmt.Errorf("failed to get the framework: %w", err)
	}
	framework.Spec = spec

	if framework.Spec.NamespaceName == "" {
		framework.Spec.NamespaceName = fmt.Sprintf("ketch-%s", framework.Spec.Name)
	}
	return &framework, nil
}

// updateFrameworkFromArgs fetches the named Framework, updates a Framework.Spec from options, and returns the Framework.
func updateFrameworkFromArgs(ctx context.Context, cfg config, options frameworkUpdateOptions) (*ketchv1.Framework, error) {
	var framework ketchv1.Framework
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.name}, &framework); err != nil {
		return nil, fmt.Errorf("failed to get the framework: %w", err)
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
		framework.Spec.IngressController.ClusterIssuer = options.ingressClusterIssuer
	}
	return &framework, nil
}
