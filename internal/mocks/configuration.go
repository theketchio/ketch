package mocks

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dynamicFake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	kubeFake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlFake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/templates"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = kubeFake.AddToScheme(scheme)
	_ = ketchv1.AddToScheme(scheme)
}

// Configuration provides a way to mock clients.
type Configuration struct {
	CtrlClientObjects    []runtime.Object
	KubeClientObjects    []runtime.Object
	DynamicClientObjects []runtime.Object
	StorageInstance      templates.Client

	ctrlClient client.Client
}

func (cfg *Configuration) Client() client.Client {
	if cfg.ctrlClient == nil {
		cfg.ctrlClient = ctrlFake.NewFakeClientWithScheme(scheme, cfg.CtrlClientObjects...)
	}
	return cfg.ctrlClient
}

func (cfg *Configuration) Storage() templates.Client {
	return cfg.StorageInstance
}

// KubernetesClient returns kubernetes typed client. It's used to work with standard kubernetes types.
func (cfg *Configuration) KubernetesClient() kubernetes.Interface {
	return kubeFake.NewSimpleClientset(cfg.KubeClientObjects...)
}

// DynamicClient returns kubernetes dynamic client. It's used to work with CRDs for which we don't have go types like ClusterIssuer.
func (cfg *Configuration) DynamicClient() dynamic.Interface {
	return dynamicFake.NewSimpleDynamicClient(runtime.NewScheme(), cfg.DynamicClientObjects...)
}
