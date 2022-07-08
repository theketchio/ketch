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
	"testing"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
	"github.com/theketchio/ketch/internal/mocks"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestIngressSet(t *testing.T) {
	mockConfigmap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: ketchv1.IngressConfigmapName, Namespace: ketchv1.IngressConfigmapNamespace},
		Data: map[string]string{
			"className":       "nginx",
			"serviceEndpoint": "127.0.0.1",
			"clusterIssuer":   "letsencrypt",
			"ingressType":     "nginx",
		},
	}
	tests := []struct {
		name    string
		cfg     config
		options ingressSetOptions
		want    string
		wantErr string
	}{
		{
			name: "successful update",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{mockConfigmap},
			},
			options: ingressSetOptions{
				ingressType: "traefik",
			},
			want: "Successfully set!\n",
		},
		{
			name: "error - missing fields",
			cfg:  &mocks.Configuration{},
			options: ingressSetOptions{
				ingressType: "traefik",
			},
			wantErr: "ingress-class-name, ingress-service-endpoint, and ingress-type are required",
		},
		{
			name: "successful create",
			cfg:  &mocks.Configuration{},
			options: ingressSetOptions{
				ingressType:     "nginx",
				className:       "nginx",
				serviceEndpoint: "127.0.0.1",
			},
			want: "Successfully set!\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			err := ingressSet(context.Background(), tt.cfg, tt.options, out)
			if tt.wantErr != "" {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.want, out.String())
		})
	}
}

func TestIngressGet(t *testing.T) {
	mockConfigmap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: ketchv1.IngressConfigmapName, Namespace: ketchv1.IngressConfigmapNamespace},
		Data: map[string]string{
			"className":       "nginx",
			"serviceEndpoint": "127.0.0.1",
			"clusterIssuer":   "letsencrypt",
			"ingressType":     "nginx",
		},
	}
	tests := []struct {
		name    string
		cfg     config
		want    string
		wantErr string
	}{
		{
			name: "success",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{mockConfigmap},
			},
			want: "Class Name: nginx\nService Endpoint: 127.0.0.1\nIngress Type: nginx\nCluster Issuer: letsencrypt\n",
		},
		{
			name:    "error - not set",
			cfg:     &mocks.Configuration{},
			wantErr: "failed to get ingress: configmaps \"ketch-ingress\" not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			err := ingressGet(context.Background(), tt.cfg, out)
			if tt.wantErr != "" {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.want, out.String())
		})
	}
}
