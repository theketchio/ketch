package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/validation"
)

const poolAddHelp = `
Add a new pool.
`

func newPoolAddCmd(cfg config, out io.Writer) *cobra.Command {
	options := poolAddOptions{}
	cmd := &cobra.Command{
		Use:   "add POOL",
		Args:  cobra.ExactValidArgs(1),
		Short: "Add a new pool.",
		Long:  poolAddHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.name = args[0]
			return addPool(cmd.Context(), cfg, options, out)
		},
	}
	cmd.Flags().StringVar(&options.kubeNamespace, "kube-namespace", "", "Kubernetes namespace for this pool")
	cmd.Flags().IntVar(&options.appQuotaLimit, "app-quota-limit", -1, "Quota limit for app when adding it to this pool")
	cmd.Flags().StringVar(&options.ingressClassName, "ingress-class-name", "", "if set, it is used as kubernetes.io/ingress.class annotations")
	cmd.Flags().StringVar(&options.ingressDomainName, "ingress-domain", "shipa.cloud", "domain name for the default URL")
	cmd.Flags().StringVar(&options.ingressServiceEndpoint, "ingress-service-endpoint", "", "an IP address or dns name of the ingress controller's Service")
	cmd.MarkFlagRequired("kube-namespace")
	return cmd
}

type poolAddOptions struct {
	name string

	appQuotaLimit int
	kubeNamespace string

	ingressClassName       string
	ingressDomainName      string
	ingressServiceEndpoint string
}

func addPool(ctx context.Context, cfg config, options poolAddOptions, out io.Writer) error {
	if !validation.ValidateName(options.name) {
		return ErrInvalidPoolName
	}
	pool := ketchv1.Pool{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: options.name,
		},
		Spec: ketchv1.PoolSpec{
			NamespaceName: options.kubeNamespace,
			AppQuotaLimit: options.appQuotaLimit,
			IngressController: ketchv1.IngressControllerSpec{
				ClassName:       options.ingressClassName,
				Domain:          options.ingressDomainName,
				ServiceEndpoint: options.ingressServiceEndpoint,
			},
		},
		Status: ketchv1.PoolStatus{},
	}
	if err := cfg.Client().Create(ctx, &pool); err != nil {
		return fmt.Errorf("failed to create pool: %w", err)
	}
	return nil
}
