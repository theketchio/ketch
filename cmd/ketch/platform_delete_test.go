package main

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/shipa-corp/ketch/internal/api/v1beta1"
)

type resourceGetterDeleterMock struct {
	resourceGetterMock
	resourceDeleterMock
}

func TestPlatformDelete(t *testing.T) {
	expected := "java"
	var mp resourceGetterDeleterMock
	mp.resourceGetterMock.f = func(nn types.NamespacedName, obj runtime.Object) error {
		require.Equal(t, expected, nn.Name)
		plat, ok := obj.(*v1beta1.Platform)
		require.True(t, ok)
		plat.ObjectMeta.Name = nn.Name
		return nil
	}
	mp.resourceDeleterMock.f = func(obj runtime.Object) error {
		plat, ok := obj.(*v1beta1.Platform)
		require.True(t, ok)
		require.Equal(t, expected, plat.ObjectMeta.Name)
		return nil
	}
	cmd := newPlatformDeleteCmd(&mp, ioutil.Discard)
	cmd.SetArgs([]string{expected})
	cmd.SetOut(ioutil.Discard)
	err := cmd.Execute()
	require.Nil(t, err)
	require.True(t, mp.resourceGetterMock.called)
	require.True(t, mp.resourceDeleterMock.called)
}
