package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"text/template"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

var (
	appInfoTemplate = `Application: {{ .App.Name }}
Description: {{ .App.Spec.Description }}
{{- range $address := .Cnames }}
Address: {{ $address }}
{{- end }}
Deploys: {{ .App.Spec.DeploymentsCount }}
Pool: {{ .App.Spec.Pool }} 
{{- if .App.Spec.Deployments }}

Routing settings: 
{{- range $deployment := .App.Spec.Deployments }}
   {{ $deployment.Version }} version => {{ $deployment.RoutingSettings.Weight }} weight
{{- end }}
{{- end }}`
)

type appInfoContext struct {
	App    ketchv1.App
	Cnames []string
}

const appInfoHelp = `
Show information about a specific app.
`

func newAppInfoCmd(cfg config, out io.Writer) *cobra.Command {
	options := appInfoOptions{}
	cmd := &cobra.Command{
		Use:   "info APPNAME",
		Short: "Show information about a specific app.",
		Args:  cobra.ExactArgs(1),
		Long:  appInfoHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.name = args[0]
			return appInfo(cmd.Context(), cfg, options, out)
		},
	}
	return cmd
}

type appInfoOptions struct {
	name string
}

func appInfo(ctx context.Context, cfg config, options appInfoOptions, out io.Writer) error {
	app := ketchv1.App{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.name}, &app); err != nil {
		return fmt.Errorf("failed to get app: %w", err)
	}
	pool := &ketchv1.Pool{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: app.Spec.Pool}, pool); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get pool: %w", err)
		}
		pool = nil
	}
	buf := bytes.Buffer{}
	t := template.Must(template.New("app-info").Parse(appInfoTemplate))
	infoContext := appInfoContext{
		App:    app,
		Cnames: app.CNames(pool),
	}
	if err := t.Execute(&buf, infoContext); err != nil {
		return err
	}
	fmt.Fprintf(out, "%v\n", buf.String())
	return nil
}
