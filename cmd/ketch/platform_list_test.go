package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestPlatformList(t *testing.T) {
	expected := `NAME    IMAGE                    DESCRIPTION
java    shipacorp/java:latest    something
`
	lister := resourceListerMock{
		f: func(obj runtime.Object) error {
			pl, ok := obj.(*v1beta1.PlatformList)
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
		},
	}

	var buff bytes.Buffer
	cmd := newPlatformListCmd(&lister, &buff)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Nil(t, err)
	require.Equal(t, expected, buff.String())
}
