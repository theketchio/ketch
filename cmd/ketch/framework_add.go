package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/thediveo/enumflag"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/validation"
)

const frameworkAddHelp = `
Add a new framework.
`

type ingressType enumflag.Flag

const (
	traefik ingressType = iota
	istio
)

const (
	defaultIstioIngressClassName   = "istio"
	defaultTraefikIngressClassName = "traefik"
	defaultVersion                 = "v1"
)

var (
	defaultAppQuotaLimit = -1
)

var ingressTypeIds = map[ingressType][]string{
	traefik: {ketchv1.TraefikIngressControllerType.String()},
	istio:   {ketchv1.IstioIngressControllerType.String()},
}

type addFrameworkFn func(ctx context.Context, cfg config, options frameworkAddOptions, out io.Writer) error

func newFrameworkAddCmd(cfg config, out io.Writer, addFramework addFrameworkFn) *cobra.Command {
	options := frameworkAddOptions{}
	cmd := &cobra.Command{
		Use:   "add FRAMEWORK",
		Args:  cobra.ExactValidArgs(1),
		Short: "Add a new framework.",
		Long:  frameworkAddHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.name = args[0]
			options.ingressClassNameSet = cmd.Flags().Changed("ingress-class-name")
			return addFramework(cmd.Context(), cfg, options, out)
		},
	}
	cmd.Flags().StringVar(&options.version, "version", defaultVersion, "Version for this framework")
	cmd.Flags().StringVar(&options.namespace, "namespace", "", "Kubernetes namespace for this framework")
	cmd.Flags().IntVar(&options.appQuotaLimit, "app-quota-limit", defaultAppQuotaLimit, "Quota limit for app when adding it to this framework")
	cmd.Flags().StringVar(&options.ingressClassName, "ingress-class-name", "", `if set, it is used as kubernetes.io/ingress.class annotations. Ketch uses "istio" class name for istio ingress controller, if class name is not specified`)
	cmd.Flags().StringVar(&options.ingressClusterIssuer, "cluster-issuer", "", "ClusterIssuer to obtain SSL certificates")
	cmd.Flags().StringVar(&options.ingressServiceEndpoint, "ingress-service-endpoint", "", "an IP address or dns name of the ingress controller's Service")
	cmd.Flags().Var(enumflag.New(&options.ingressType, "ingress-type", ingressTypeIds, enumflag.EnumCaseInsensitive), "ingress-type", "ingress controller type: traefik or istio")
	return cmd
}

type frameworkAddOptions struct {
	version string
	name    string

	appQuotaLimit int
	namespace     string

	ingressClassNameSet    bool
	ingressClassName       string
	ingressClusterIssuer   string
	ingressServiceEndpoint string
	ingressType            ingressType
}

func addFramework(ctx context.Context, cfg config, options frameworkAddOptions, out io.Writer) error {
	var framework *ketchv1.Framework
	var err error

	switch {
	case validation.ValidateYamlFilename(options.name):
		framework, err = newFrameworkFromYaml(options)
		if err != nil {
			return err
		}
	case validation.ValidateName(options.name):
		framework = newFrameworkFromArgs(options)
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
	fmt.Println(framework.Spec.Version)

	if err := cfg.Client().Create(ctx, framework); err != nil {
		return fmt.Errorf("failed to create framework: %w", err)
	}
	fmt.Fprintln(out, "Successfully added!")
	return nil
}

// newFrameworkFromYaml imports a Framework definition from a yaml file specified in options.name.
// It asserts that the framework has a name. It assigns a ketch-prefixed namespaceName, version,
// ingressController className, and ingressController type (defaulting to traefik) if values are not specified.
func newFrameworkFromYaml(options frameworkAddOptions) (*ketchv1.Framework, error) {
	f, err := os.Open(options.name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var framework ketchv1.Framework
	err = yaml.NewDecoder(f).Decode(&framework.Spec)
	if err != nil {
		return nil, err
	}
	if len(framework.Spec.Name) == 0 {
		return nil, errors.New("a framework name is required")
	}
	framework.ObjectMeta.Name = framework.Spec.Name
	if framework.Spec.NamespaceName == "" {
		framework.Spec.NamespaceName = fmt.Sprintf("ketch-%s", framework.Spec.Name)
	}
	if framework.Spec.Version == "" {
		framework.Spec.Version = defaultVersion
	}
	if framework.Spec.AppQuotaLimit == nil {
		framework.Spec.AppQuotaLimit = &defaultAppQuotaLimit
	}
	if len(framework.Spec.IngressController.IngressType) == 0 {
		framework.Spec.IngressController.IngressType = ketchv1.TraefikIngressControllerType
	}
	if len(framework.Spec.IngressController.ClassName) == 0 {
		if framework.Spec.IngressController.IngressType.String() == defaultIstioIngressClassName {
			framework.Spec.IngressController.ClassName = defaultIstioIngressClassName
		} else {
			framework.Spec.IngressController.ClassName = defaultTraefikIngressClassName
		}
	}
	return &framework, nil
}

// newFrameworkFromArgs creates a Framework from options. It creates a ketch-prefixed namespace if
// one is not specified.
func newFrameworkFromArgs(options frameworkAddOptions) *ketchv1.Framework {
	namespace := fmt.Sprintf("ketch-%s", options.name)
	if len(options.namespace) > 0 {
		namespace = options.namespace
	}

	framework := &ketchv1.Framework{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: options.name,
		},
		Spec: ketchv1.FrameworkSpec{
			NamespaceName: namespace,
			AppQuotaLimit: &options.appQuotaLimit,
			IngressController: ketchv1.IngressControllerSpec{
				ClassName:       options.IngressClassName(),
				ServiceEndpoint: options.ingressServiceEndpoint,
				ClusterIssuer:   options.ingressClusterIssuer,
				IngressType:     options.ingressType.ingressControllerType(),
			},
		},
		Status: ketchv1.FrameworkStatus{},
	}
	return framework
}

func (o frameworkAddOptions) IngressClassName() string {
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
