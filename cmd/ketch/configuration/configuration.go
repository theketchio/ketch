package configuration

import (
	"log"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/controllers"
	"github.com/shipa-corp/ketch/internal/templates"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = ketchv1.AddToScheme(scheme)
}

// Configuration provides methods to get initialized clients.
type Configuration struct {
	cli     client.Client
	storage *templates.Storage
}

// Client returns initialized controller-runtime's Client to perform CRUD operations on Kubernetes objects.
func (cfg *Configuration) Client() client.Client {
	if cfg.cli != nil {
		return cfg.cli
	}
	configFlags := genericclioptions.NewConfigFlags(true)
	factory := cmdutil.NewFactory(configFlags)
	kubeCfg, err := factory.ToRESTConfig()
	if err != nil {
		log.Fatalf("failed to create kubernetes client: %v", err)
	}
	cfg.cli, err = client.New(kubeCfg, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatalf("failed to create kubernetes client: %v", err)
	}
	return cfg.cli
}

// KubernetesClient returns kubernetes typed client. It's used to work with standard kubernetes types.
func (cfg *Configuration) KubernetesClient() kubernetes.Interface {
	configFlags := genericclioptions.NewConfigFlags(true)
	factory := cmdutil.NewFactory(configFlags)
	kubeCfg, err := factory.ToRESTConfig()
	if err != nil {
		log.Fatalf("failed to create kubernetes client: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(kubeCfg)
	if err != nil {
		log.Fatalf("failed to create kubernetes client: %v", err)
	}
	return clientset
}

// Client returns initialized templates.Client to perform CRUD operations on templates.
func (cfg *Configuration) Storage() templates.Client {
	if cfg.storage != nil {
		return cfg.storage
	}
	cfg.storage = templates.NewStorage(cfg.Client(), controllers.KetchNamespace)
	return cfg.storage
}

// DynamicClient returns kubernetes dynamic client. It's used to work with CRDs for which we don't have go types like ClusterIssuer.
func (cfg *Configuration) DynamicClient() dynamic.Interface {
	flags := genericclioptions.NewConfigFlags(true)
	factory := cmdutil.NewFactory(flags)
	conf, err := factory.ToRESTConfig()
	if err != nil {
		log.Fatalf("failed to create kubernetes client: %v", err)
	}
	i, err := dynamic.NewForConfig(conf)
	if err != nil {
		log.Fatalf("failed to create kubernetes client: %v", err)
	}
	return i
}
