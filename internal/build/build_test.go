package build

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/moby/buildkit/client"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

type mockGetter struct{}

func (m *mockGetter) Get(ctx context.Context, name types.NamespacedName, object runtime.Object) error {
	switch crd := object.(type) {
	case *ketchv1.App:
		crd.Spec.Platform = "go"
		return nil
	case *ketchv1.Platform:
		crd.Spec.Image = "shipasoftware/go:v1.2"
		return nil
	}
	return errors.New("type not found")
}

func TestBuildContext(t *testing.T) {

	bc, err := newBuildContext()
	require.Nil(t, err)
	dockerFile, err := bc.getDockerfile("shipasoftware/go:v1.2",
		"/home/jam/go/src/github.com/shipa-corp/go-sample",
		[]string{"."},
	)
	require.Nil(t, err)
	t.Log(dockerFile)

}

type mockBuildResourceGetter struct{}

func (mb *mockBuildResourceGetter) Get(ctx context.Context, name types.NamespacedName, object runtime.Object) error {
	switch v := object.(type) {
	case *ketchv1.App:
		v.Spec.Platform = "myplatform"
		return nil
	case *ketchv1.Platform:
		v.Spec.Image = "shipasoftware/go:v1.2"
		return nil
	}
	return fmt.Errorf("unknown object %s %v", name.String(), object)
}

func TestGetSourceHandler(t *testing.T) {
	ctx := context.Background()
	cli, err := client.New(ctx, "")
	require.Nil(t, err)
	var req CreateImageFromSourceRequest
	req.ImageURI = "murphybytes/zippy:v0.1"
	req.AppName = "myapp"

	res, err := GetSourceHandler(cli, &mockBuildResourceGetter{})(
		ctx,
		&req,
		WithWorkingDirectory("/home/jam/go/src/github.com/shipa-corp/go-sample"),
		)
	require.Nil(t, err)
	t.Logf(">>> %q", res.ImageURI)
}
