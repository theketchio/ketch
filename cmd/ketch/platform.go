package main

import (
	"context"
	"io"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/shipa-corp/ketch/internal/api/v1beta1"
)

func newPlatformCmd(cfg config, logOut io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "platform COMMAND",
		Short: "Manage platforms",
		Long:  "Adds and removes platforms to build your apps.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(newPlatformAddCmd(cfg.Client(), logOut))
	cmd.AddCommand(newPlatformListCmd(cfg.Client(), logOut))
	cmd.AddCommand(newPlatformDeleteCmd(cfg.Client(), logOut))

	return cmd
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
