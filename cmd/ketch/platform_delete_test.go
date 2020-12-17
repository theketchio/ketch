package main

import (
	"context"
	"github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockPlatformDeleter struct {
	getCalled    bool
	getFn        func(name types.NamespacedName, obj runtime.Object)
	deleteCalled bool
	deleteFn     func(obj runtime.Object)
}

func (m *mockPlatformDeleter) Get(ctx context.Context, nn types.NamespacedName, object runtime.Object) error {
	m.getCalled = true
	m.getFn(nn, object)
	return nil
}

func (m *mockPlatformDeleter) Delete(ctx context.Context, object runtime.Object, option ...client.DeleteOption) error {
	m.deleteCalled = true
	m.deleteFn(object)
	return nil
}

func TestPlatformDelete(t *testing.T) {
	expected := "java"
	var mp mockPlatformDeleter
	mp.getFn = func(nn types.NamespacedName, obj runtime.Object) {
		require.Equal(t, expected, nn.Name)
		plat, ok := obj.(*v1beta1.Platform)
		require.True(t, ok)
		plat.ObjectMeta.Name = nn.Name
	}
	mp.deleteFn = func(obj runtime.Object) {
		plat, ok := obj.(*v1beta1.Platform)
		require.True(t, ok)
		require.Equal(t, expected, plat.ObjectMeta.Name)
	}
	cmd := newPlatformDeleteCmd(&mp, ioutil.Discard)
	cmd.SetArgs([]string{expected})
	cmd.SetOut(ioutil.Discard)
	err := cmd.Execute()
	require.Nil(t, err)
	require.True(t, mp.getCalled)
	require.True(t, mp.deleteCalled)
}
