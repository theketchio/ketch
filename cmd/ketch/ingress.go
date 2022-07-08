/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"text/template"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"

	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ingressSetOptions struct {
	className       string
	serviceEndpoint string
	ingressType     string
	clusterIssuer   string
}

func newIngressCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ingress",
		Short: "Manage Ingress",
		Long:  "Manage Ingress",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(newIngressSetCmd(cfg, out))
	cmd.AddCommand(newIngressGetCmd(cfg, out))
	return cmd
}

const ingressSetHelp = `Creates/Updates the ingress that apps will use.
Ingress is stored in a configmap's data, for example:
data:
  className: nginx #required
  ingressType: nginx #required
  serviceEndpoint: 127.0.0.1 #required
  clusterIssuer: letsencrypt
`

var ingressSetValidationError = fmt.Errorf("ingress-class-name, ingress-service-endpoint, and ingress-type are required")

func newIngressSetCmd(cfg config, out io.Writer) *cobra.Command {
	var options ingressSetOptions

	cmd := &cobra.Command{
		Use:   "set [--ingress-class-name/-c <class_name>] [--ingress-service-endpoint/-s <service_endpoint>] [--ingress-type/-t <type>] [--cluster-issuer <cluster_issuer>]",
		Short: "Set ingress controller values",
		Long:  ingressSetHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ingressSet(cmd.Context(), cfg, options, out)
		},
	}
	cmd.Flags().StringVarP(&options.className, "ingress-class-name", "c", "", "If set, it is used as kubernetes.io/ingress.class annotations")
	cmd.Flags().StringVarP(&options.serviceEndpoint, "ingress-service-endpoint", "s", "", "An IP address or DNS name of the ingress controller's Service")
	cmd.Flags().StringVarP(&options.ingressType, "ingress-type", "t", "", "Ingress controller type: nginx, traefik, istio")
	cmd.Flags().StringVar(&options.clusterIssuer, "cluster-issuer", "", "ClusterIssuer to obtain SSL certificates")

	return cmd
}

func ingressSet(ctx context.Context, cfg config, options ingressSetOptions, out io.Writer) error {
	configmap := v1.ConfigMap{}
	err := cfg.Client().Get(ctx, types.NamespacedName{Name: ketchv1.IngressConfigmapName, Namespace: ketchv1.IngressConfigmapNamespace}, &configmap)
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to get ingress: %w", err)
	}
	if configmap.Data == nil {
		configmap.Data = make(map[string]string)
	}
	if options.className != "" {
		configmap.Data["className"] = options.className
	}
	if options.serviceEndpoint != "" {
		configmap.Data["serviceEndpoint"] = options.serviceEndpoint
	}
	if options.ingressType != "" {
		configmap.Data["ingressType"] = options.ingressType
	}
	if options.clusterIssuer != "" {
		configmap.Data["clusterIssuer"] = options.clusterIssuer
	}
	if val, ok := configmap.Data["className"]; !ok || val == "" {
		return ingressSetValidationError
	}
	if val, ok := configmap.Data["serviceEndpoint"]; !ok || val == "" {
		return ingressSetValidationError
	}
	if val, ok := configmap.Data["ingressType"]; !ok || val == "" {
		return ingressSetValidationError
	}
	if err != nil {
		// create
		configmap.Name = ketchv1.IngressConfigmapName
		configmap.Namespace = ketchv1.IngressConfigmapNamespace
		if err := cfg.Client().Create(ctx, &configmap); err != nil {
			return fmt.Errorf("failed to create ingress: %w", err)
		}
	} else {
		// update
		if err := cfg.Client().Update(ctx, &configmap); err != nil {
			return fmt.Errorf("failed to set ingress: %w", err)
		}
	}
	fmt.Fprintln(out, "Successfully set!")
	return nil
}

var (
	ingressGetTemplate = `Class Name: {{ .className }}
Service Endpoint: {{ .serviceEndpoint }}
Ingress Type: {{ .ingressType }}
{{- if .clusterIssuer }}
Cluster Issuer: {{ .clusterIssuer }}
{{- end }}
`
)

func newIngressGetCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get ingress controller values",
		Long:  "Get ingress controller values",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ingressGet(cmd.Context(), cfg, out)
		},
	}
	return cmd
}

func ingressGet(ctx context.Context, cfg config, out io.Writer) error {
	configmap := v1.ConfigMap{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: ketchv1.IngressConfigmapName, Namespace: ketchv1.IngressConfigmapNamespace}, &configmap); err != nil {
		return fmt.Errorf("failed to get ingress: %w", err)
	}

	var buf bytes.Buffer
	t := template.Must(template.New("ingress-get").Parse(ingressGetTemplate))
	if err := t.Execute(&buf, configmap.Data); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "%v", buf.String())
	return err
}
