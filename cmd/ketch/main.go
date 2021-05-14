package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/pflag"
	_ "k8s.io/client-go/plugin/pkg/client/auth/exec"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"github.com/shipa-corp/ketch/cmd/ketch/configuration"
)

var (
	// version is set by goreleaser.
	version = "dev"
)

func main() {
	// Remove any flags that were added by libraries automatically.
	pflag.CommandLine = pflag.NewFlagSet("ketch", pflag.ExitOnError)

	cmd := newRootCmd(initConfig(), os.Stdout)
	if err := cmd.Execute(); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}

func initConfig() *configuration.Configuration {
	path, err := configuration.DefaultConfigPath()
	if err != nil {
		log.Println(err)
		return &configuration.Configuration{}
	}

	cfg, err := configuration.Read(path)
	if err != nil {
		log.Println(err)
		return &configuration.Configuration{}
	}

	return cfg
}
