package main

import (
	"context"
	"io"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/shipa-corp/ketch/internal/api/v1beta1"
)

func newPlatformCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "platform COMMAND",
		Short: "Manage platforms",
		Long:  "Adds and removes platforms to build your apps.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(newPlatformAddCmd(jit(cfg), out))
	cmd.AddCommand(newPlatformListCmd(jit(cfg), out))
	cmd.AddCommand(newPlatformDeleteCmd(jit(cfg), out))

	return cmd
}

type justInTimeClient struct {
	cfg clientCtor
}

type clientCtor interface {
	Client() client.Client
}

// defers the creation of the client until we need it (Just In Time). The reason we do this is so that the application doesn't
// attempt to connect to a k8s cluster unless we need to in order to avoid delays in say, showing help.
func jit(cfg clientCtor) *justInTimeClient {
	return &justInTimeClient{
		cfg: cfg,
	}
}

func (rg *justInTimeClient) Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
	return rg.cfg.Client().Create(ctx, obj, opts...)
}

func (rg *justInTimeClient) Get(ctx context.Context, name types.NamespacedName, object runtime.Object) error {
	return rg.cfg.Client().Get(ctx, name, object)
}

func (rg *justInTimeClient) Delete(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error {
	return rg.cfg.Client().Delete(ctx, obj, opts...)
}

func (rg *justInTimeClient) List(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
	return rg.cfg.Client().List(ctx, list, opts...)
}

func platformGet(ctx context.Context, getter resourceGetter, platformName string) (*v1beta1.Platform, error) {
	var platform v1beta1.Platform
	if err := getter.Get(ctx, types.NamespacedName{Name: platformName}, &platform); err != nil {
		return nil, err
	}
	return &platform, nil
}

func platformList(ctx context.Context, lister resourceLister) (*v1beta1.PlatformList, error) {
	var list v1beta1.PlatformList
	if err := lister.List(ctx, &list); err != nil {
		return nil, err
	}

	return &list, nil
}

type platformSpec struct {
	name        string
	image       string
	description string
}

func platformCreate(ctx context.Context, creator resourceCreator, ps platformSpec) error {
	platform := v1beta1.Platform{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: ps.name,
		},
		Spec: v1beta1.PlatformSpec{
			Image:       ps.image,
			Description: ps.description,
		},
	}
	return creator.Create(ctx, &platform)
}

func platformDelete(ctx context.Context, deleter resourceDeleter, platform *v1beta1.Platform) error {
	return deleter.Delete(ctx, platform)
}
