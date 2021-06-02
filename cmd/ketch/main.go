package main

import (
	"log"
	"os"

	"github.com/spf13/pflag"
	_ "k8s.io/client-go/plugin/pkg/client/auth/exec"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"github.com/shipa-corp/ketch/cmd/ketch/configuration"
	"github.com/shipa-corp/ketch/internal/pack"
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

	cmd := newRootCmd(&configuration.Configuration{}, out, packSvc)
	if err := cmd.Execute(); err != nil {
		log.Fatalf("execution failed %q", err)
	}
}
