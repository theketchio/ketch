package chart

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/postrender"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/krusty"
	kTypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

var _ postrender.PostRenderer = &postRender{}

type postRender struct {
	log logr.Logger
	cli client.Client

	appId              string
	appName            string
	deploymentVersions []int
	namespace          string
}

func (p *postRender) Run(renderedManifests *bytes.Buffer) (modifiedManifests *bytes.Buffer, err error) {
	var configMapList v1.ConfigMapList
	opts := &client.ListOptions{Namespace: p.namespace}
	if err := p.cli.List(context.Background(), &configMapList, opts); err != nil {
		return nil, err
	}

	finalBuffer := renderedManifests
	for _, cm := range configMapList.Items {
		fwPatch := strings.HasSuffix(cm.Name, "-postrender")
		var appPatch bool
		for _, dv := range p.deploymentVersions {
			appPatch = strings.HasPrefix(cm.Name, fmt.Sprintf("%s-%d-app-post-render", p.appName, dv)) ||
				strings.HasPrefix(cm.Name, fmt.Sprintf("%s-%s-%d-app-post-render", p.appName, p.appId, dv))
			if appPatch {
				break
			}
		}

		if !fwPatch && !appPatch {
			continue
		}
		p.log.Info(fmt.Sprintf("including post renderer patch: appPatch: %t, fwPatch %t, %s ", appPatch, fwPatch, cm.Name))

		fs := filesys.MakeFsInMemory()
		localPath := p.localPath(fwPatch, p.appName)
		if !fs.Exists(localPath) {
			if err := fs.Mkdir(localPath); err != nil {
				return nil, err
			}
		}

		for k, v := range cm.Data {
			fileName := localPath + "/" + k
			if err := fs.WriteFile(fileName, []byte(v)); err != nil {
				return nil, err
			}
		}
		if err := fs.WriteFile(localPath+"/app.yaml", finalBuffer.Bytes()); err != nil {
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

		result, err := kustomizer.Run(fs, localPath)
		if err != nil {
			return nil, err
		}
		y, err := result.AsYaml()
		if err != nil {
			return nil, err
		}
		finalBuffer = bytes.NewBuffer(y)
		if err := fs.WriteFile(localPath+"/app.yaml", finalBuffer.Bytes()); err != nil {
			return nil, err
		}
	}

	return finalBuffer, nil
}

func (p postRender) localPath(fwPatch bool, name string) string {
	if fwPatch {
		return p.namespace
	}
	return p.appName
}
