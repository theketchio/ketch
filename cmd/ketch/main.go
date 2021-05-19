package main

import (
	"log"
	"os"

	"github.com/spf13/pflag"
	_ "k8s.io/client-go/plugin/pkg/client/auth/exec"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"github.com/shipa-corp/ketch/cmd/ketch/configuration"
	"github.com/shipa-corp/ketch/internal/docker"
)

var (
	// version is set by goreleaser.
	version = "dev"
)

func main() {
	// Remove any flags that were added by libraries automatically.
	pflag.CommandLine = pflag.NewFlagSet("ketch", pflag.ExitOnError)

	dockerSvc, err := docker.New()
	if err != nil {
		log.Fatalf("couldn't create docker service %q", err)
	}
	defer dockerSvc.Close()

	cmd := newRootCmd(&configuration.Configuration{}, os.Stdout, dockerSvc)
	if err := cmd.Execute(); err != nil {
		log.Fatalf("execution failed %q", err)
	}
}
