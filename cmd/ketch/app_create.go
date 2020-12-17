package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/validation"
)

const appCreateHelp = `
Creates a new app using the given name.
`

func newAppCreateCmd(cfg config, out io.Writer) *cobra.Command {
	options := appCreateOptions{}
	cmd := &cobra.Command{
		Use:   "create APPNAME",
		Short: "Create an app",
		Long:  appCreateHelp,
		Args:  cobra.ExactValidArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.name = args[0]
			return appCreate(cmd.Context(), cfg, options, out)
		},
	}

	cmd.Flags().StringVarP(&options.platform, "platform", "P", "", "Platform name")
	cmd.Flags().StringVarP(&options.description, "description", "d", "", "App description")
	cmd.Flags().StringSliceVarP(&options.envs, "env", "e", []string{}, "App env variables")
	cmd.Flags().StringVarP(&options.pool, "pool", "o", "", "Pool to deploy your app")
	cmd.Flags().StringVarP(&options.dockerRegistrySecret, "registry-secret", "", "", "A name of a Secret with docker credentials. This secret must be created in the same namespace of the pool.")
	cmd.MarkFlagRequired("pool")

	return cmd
}

type appCreateOptions struct {
	name                 string
	pool                 string
	description          string
	envs                 []string
	dockerRegistrySecret string
	platform             string
}

func appCreate(ctx context.Context, cfg config, options appCreateOptions, out io.Writer) error {
	if !validation.ValidateName(options.name) {
		return ErrInvalidAppName
	}
	envs, err := getEnvs(options.envs)
	if err != nil {
		return fmt.Errorf("failed to parse env variables: %w", err)
	}
	if options.platform != "" {
		if _, err := platformGet(ctx, cfg.Client(), options.platform); err != nil {
			return fmt.Errorf("unable to add platform %q to app: %w", options.platform, err)
		}
	}
	app := ketchv1.App{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: options.name,
		},
		Spec: ketchv1.AppSpec{
			Description: options.description,
			Deployments: []ketchv1.AppDeploymentSpec{},
			Env:         envs,
			Pool:        options.pool,
			Ingress: ketchv1.IngressSpec{
				GenerateDefaultCname: true,
			},
			DockerRegistry: ketchv1.DockerRegistrySpec{
				SecretName: options.dockerRegistrySecret,
			},
			Platform: options.platform,
		},
	}
	var pool ketchv1.Pool
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: app.Spec.Pool}, &pool); err != nil {
		return fmt.Errorf("failed to get pool instance: %w", err)
	}
	if err = cfg.Client().Create(ctx, &app); err != nil {
		return fmt.Errorf("failed to create an app: %w", err)
	}
	return nil
}
