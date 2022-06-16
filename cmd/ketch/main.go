package main

import (
	"log"
	"os"

	"github.com/spf13/pflag"
	_ "k8s.io/client-go/plugin/pkg/client/auth/exec"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"github.com/theketchio/ketch/cmd/ketch/configuration"
	"github.com/theketchio/ketch/internal/pack"
)

var (
	// version is set by goreleaser.
	version = "dev"
)

func main() {
	// Remove any flags that were added by libraries automatically.
	pflag.CommandLine = pflag.NewFlagSet("ketch", pflag.ExitOnError)

	out := os.Stdout
	packSvc, err := pack.New(out)
	if err != nil {
		log.Fatalf("couldn't create pack service %q", err)
	}

	cmd := newRootCmd(&configuration.Configuration{}, out, packSvc, getKetchConfig())
	if err := cmd.Execute(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

// the KetchConfig is optional and in the event it is not found an empty one is returned
func getKetchConfig() configuration.KetchConfig {
	path, err := configuration.DefaultConfigPath()
	if err != nil {
		log.Println(err)
		return configuration.KetchConfig{}
	}

	return configuration.Read(path)
}
