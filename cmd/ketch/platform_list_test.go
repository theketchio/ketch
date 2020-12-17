package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/shipa-corp/ketch/internal/api/v1beta1"
)

func TestPlatformList(t *testing.T) {
	expected := `+------+-----------------------+-------------+
| NAME |         IMAGE         | DESCRIPTION |
+------+-----------------------+-------------+
| java | shipacorp/java:latest | something   |
+------+-----------------------+-------------+
`
	var mock mockPlatformLister
	var buff bytes.Buffer
	cmd := newPlatformListCmd(&mock, &buff)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Nil(t, err)
	require.Equal(t, expected, buff.String())

}

type mockPlatformLister struct {
}

func (m mockPlatformLister) List(_ context.Context, o runtime.Object, _ ...client.ListOption) error {
	pl, ok := o.(*v1beta1.PlatformList)
	if !ok {
		return errors.New("can't cast to platform list")
	}
	p := v1beta1.Platform{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "java",
		},
		Spec: v1beta1.PlatformSpec{
			Image:       "shipacorp/java:latest",
			Description: "something",
		},
	}
	pl.Items = append(pl.Items, p)

	return nil
}
