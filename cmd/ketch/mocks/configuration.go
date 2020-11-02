package mocks

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeFake "k8s.io/client-go/kubernetes/fake"
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
	CtrlClientObjects []runtime.Object
	KubeClientObjects []runtime.Object

	ctrlClient client.Client
}

func (cfg *Configuration) Client() client.Client {
	if cfg.ctrlClient == nil {
		cfg.ctrlClient = ctrlFake.NewFakeClientWithScheme(scheme, cfg.CtrlClientObjects...)
	}
	return cfg.ctrlClient
}

func (cfg *Configuration) Storage() templates.Client {
	panic("not implemented")
}

func (cfg *Configuration) KubernetesClient() kubernetes.Interface {
	return kubeFake.NewSimpleClientset(cfg.KubeClientObjects...)
}
