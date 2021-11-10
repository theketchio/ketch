package chart

import (
	"bytes"
	"context"
	"fmt"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/postrender"
	"helm.sh/helm/v3/pkg/release"
	v1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"log"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	kTypes "sigs.k8s.io/kustomize/api/types"
	"strings"
)

// HelmClient performs helm install and uninstall operations for provided application helm charts.
type HelmClient struct {
	cfg       *action.Configuration
	namespace string
	c         client.Client
}

// TemplateValuer is an interface that permits types that implement it (e.g. Application, Job)
// to be parameters in the UpdateChart function.
type TemplateValuer interface {
	GetValues() interface{}
	GetTemplates() map[string]string
	GetName() string
}

// NewHelmClient returns a HelmClient instance.
func NewHelmClient(namespace string, c client.Client) (*HelmClient, error) {
	cfg, err := getActionConfig(namespace)
	if err != nil {
		return nil, err
	}
	return &HelmClient{cfg: cfg, namespace: namespace, c: c}, nil
}

func getActionConfig(namespace string) (*action.Configuration, error) {
	actionConfig := new(action.Configuration)

	config := ctrl.GetConfigOrDie()

	// Create the ConfigFlags struct instance with initialized values from ServiceAccount
	kubeConfig := genericclioptions.NewConfigFlags(false)
	kubeConfig.APIServer = &config.Host
	kubeConfig.BearerToken = &config.BearerToken
	kubeConfig.CAFile = &config.CAFile
	kubeConfig.Namespace = &namespace
	if err := actionConfig.Init(kubeConfig, namespace, os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return nil, err
	}
	return actionConfig, nil
}

// InstallOption to perform additional configuration of action.Install before running a chart installation.
type InstallOption func(install *action.Install)

type postRender struct {
	client.Client
	namespace string
}

func (p *postRender) Run(renderedManifests *bytes.Buffer) (modifiedManifests *bytes.Buffer, err error) {

	var configMapList v1.ConfigMapList
	if err := p.Client.List(context.Background(), &configMapList); err != nil {
		return nil, err
	}

	fs := filesys.MakeFsInMemory()
	if err := fs.Mkdir(p.namespace); err != nil {
		return nil, err
	}

	var postrenderFound bool
	for _, cm := range configMapList.Items {
		// only apply the postrenders present in the current namespace
		if cm.Namespace == p.namespace {
			split := strings.Split(cm.Name, "-")
			// check if a postrender is available
			if split[len(split)-1] == "postrender" {
				postrenderFound = true
				for k, v := range cm.Data {
					fileName := p.namespace + "/" + k
					if err := fs.WriteFile(fileName, []byte(v)); err != nil {
						return nil, err
					}
				}
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

var _ postrender.PostRenderer = &postRender{}

// UpdateChart checks if the app chart is already installed and performs "helm install" or "helm update" operation.
func (c HelmClient) UpdateChart(tv TemplateValuer, config ChartConfig, opts ...InstallOption) (*release.Release, error) {
	appName := tv.GetName()
	files, err := bufferedFiles(config, tv.GetTemplates(), tv.GetValues())
	if err != nil {
		return nil, err
	}
	chrt, err := loader.LoadFiles(files)
	if err != nil {
		return nil, err
	}
	vals, err := getValuesMap(tv.GetValues())
	if err != nil {
		return nil, err
	}
	getValuesClient := action.NewGetValues(c.cfg)
	getValuesClient.AllValues = true
	_, err = getValuesClient.Run(appName)
	if err != nil && err.Error() == "release: not found" {
		clientInstall := action.NewInstall(c.cfg)
		clientInstall.ReleaseName = appName
		clientInstall.Namespace = c.namespace
		clientInstall.PostRenderer = &postRender{
			namespace: c.namespace,
			Client:    c.c,
		}
		for _, opt := range opts {
			opt(clientInstall)
		}
		fmt.Println("new!!!!!!")
		return clientInstall.Run(chrt, vals)
	}
	if err != nil {
		return nil, err
	}
	updateClient := action.NewUpgrade(c.cfg)
	updateClient.Namespace = c.namespace
	updateClient.PostRenderer = &postRender{
		namespace: c.namespace,
		Client:    c.c,
	}
	fmt.Println("updates!!!!!")
	return updateClient.Run(appName, chrt, vals)
}

// DeleteChart uninstalls the app's helm release. It doesn't return an error if the release is not found.
func (c HelmClient) DeleteChart(appName string) error {
	uninstall := action.NewUninstall(c.cfg)
	_, err := uninstall.Run(appName)
	if err != nil && err.Error() == "release: not found" {
		return nil
	}
	return err
}
