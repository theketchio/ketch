package chart

import (
	"bytes"
	_ "embed"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//go:embed testdata/render_yamls/prerendered-manifests.yaml
var prerender []byte

//go:embed testdata/render_yamls/nodeaffinity-kustomization.yaml
var kustomizationNodeAffinityYaml string

//go:embed testdata/render_yamls/nodeaffinity-patch.yaml
var nodeaffinityPatchYaml string

//go:embed testdata/render_yamls/nodeaffinity-postrender.yaml
var nodeaffinityPostrender string

//go:embed testdata/render_yamls/annotations-kustomization.yaml
var kustomizationAnnotationsYaml string

//go:embed testdata/render_yamls/annotations-patch.yaml
var annotationsPatchYaml string

//go:embed testdata/render_yamls/annotations-postrender.yaml
var annotationsPostrender string

func TestPostRenderRun(t *testing.T) {
	tc := []struct {
		name      string
		configmap corev1.ConfigMap
		expected  string
	}{
		{
			name: "postrender not found",
			configmap: corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-other-configmap",
					Namespace: "fake",
				},
			},
			expected: string(prerender),
		},
		{
			name: "postrender found, patch nodeAffinity",
			configmap: corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nodeaffinity-postrender",
					Namespace: "fake",
				},
				Data: map[string]string{
					"kustomization.yaml": kustomizationNodeAffinityYaml,
					"patch.yaml":         nodeaffinityPatchYaml,
				},
			},
			expected: nodeaffinityPostrender,
		},
		{
			name: "postrender found, patch annotations",
			configmap: corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nodeaffinity-postrender",
					Namespace: "fake",
				},
				Data: map[string]string{
					"kustomization.yaml":                kustomizationAnnotationsYaml,
					"patch-annotations-Deployment.yaml": annotationsPatchYaml,
				},
			},
			expected: annotationsPostrender,
		},
	}
	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder()
			client.WithObjects(&tt.configmap)
			pr := postRender{
				log:       log.NullLogger{},
				namespace: "fake",
				cli:       client.Build(),
			}
			result, err := pr.Run(bytes.NewBuffer(prerender))
			fmt.Println(result.String())
			require.Nil(t, err)
			require.Equal(t, tt.expected, result.String())
		})
	}
}
