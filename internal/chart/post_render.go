package chart

import (
	"bytes"
	"context"
	"strings"

	"helm.sh/helm/v3/pkg/postrender"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/krusty"
	kTypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

var _ postrender.PostRenderer = &postRender{}

type postRender struct {
	cli       client.Client
	namespace string
}

func (p *postRender) Run(renderedManifests *bytes.Buffer) (modifiedManifests *bytes.Buffer, err error) {

	var configMapList v1.ConfigMapList
	opts := &client.ListOptions{Namespace: p.namespace}
	if err := p.cli.List(context.Background(), &configMapList, opts); err != nil {
		return nil, err
	}

	fs := filesys.MakeFsInMemory()
	if err := fs.Mkdir(p.namespace); err != nil {
		return nil, err
	}

	var postrenderFound bool
	for _, cm := range configMapList.Items {
		if !strings.HasSuffix(cm.Name, "-postrender") {
			continue
		}
		postrenderFound = true

		for k, v := range cm.Data {
			fileName := p.namespace + "/" + k
			if err := fs.WriteFile(fileName, []byte(v)); err != nil {
				return nil, err
			}
		}
	}

	// return original manifests, otherwise begin postrender
	if !postrenderFound {
		return renderedManifests, nil
	}

	if err := fs.WriteFile(p.namespace+"/app.yaml", renderedManifests.Bytes()); err != nil {
		return nil, err
	}

	kustomizer := krusty.MakeKustomizer(&krusty.Options{
		PluginConfig: &kTypes.PluginConfig{
			HelmConfig: kTypes.HelmConfig{
				Enabled: true,
				Command: "helm",
			},
		},
	})

	result, err := kustomizer.Run(fs, p.namespace)
	if err != nil {
		return nil, err
	}
	y, err := result.AsYaml()
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(y), nil
}
