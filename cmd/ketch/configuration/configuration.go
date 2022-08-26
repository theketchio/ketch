package configuration

import (
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1"
	"github.com/theketchio/ketch/internal/controllers"
	"github.com/theketchio/ketch/internal/templates"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = ketchv1.AddToScheme()(scheme)
}

// Configuration provides methods to get initialized clients.
type Configuration struct {
	cli     client.Client
	storage *templates.Storage
}

// KetchConfig contains all the values present in the config.toml
type KetchConfig struct {
	AdditionalBuilders []AdditionalBuilder `toml:"additional-builders,omitempty"`
	DefaultBuilder     string              `toml:"default-builder,omitempty"`
}

// AdditionalBuilder contains the information of any user added builders
type AdditionalBuilder struct {
	Vendor      string `toml:"vendor" json:"vendor" yaml:"vendor"`
	Image       string `toml:"image" json:"image" yaml:"image"`
	Description string `toml:"description" json:"description" yaml:"description"`
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

// DefaultConfigPath returns the path to the config.toml file
func DefaultConfigPath() (string, error) {
	home, err := ketchHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "config.toml"), nil
}

func ketchHome() (string, error) {
	ketchHome := os.Getenv("KETCH_HOME")
	if ketchHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		ketchHome = filepath.Join(home, ".ketch")
	}
	return ketchHome, nil
}

// Read returns a Configuration containing the unmarshalled config.toml file contents
func Read(path string) KetchConfig {
	var ketchConfig KetchConfig

	_, err := toml.DecodeFile(path, &ketchConfig)
	if err != nil && !os.IsNotExist(err) {
		return KetchConfig{}
	}
	return ketchConfig
}

//Write writes the provided KetchConfig to the given path. In the event the path is not found it will be created
func Write(ketchConfig KetchConfig, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	w, err := os.Create(path)
	if err != nil {
		return err
	}
	defer w.Close()

	return toml.NewEncoder(w).Encode(ketchConfig)
}
