package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

func Test_appDeployOptions_KetchYaml(t *testing.T) {
	tests := []struct {
		name           string
		opts           appDeployOptions
		want           *ketchv1.KetchYamlData
		wantErr        bool
		wantErrMessage string
	}{
		{
			name: "valid ketch.yaml",
			opts: appDeployOptions{
				strictKetchYamlDecoding: true,
				ketchYamlFileName:       "./testdata/ketch.yaml",
			},
			want: &ketchv1.KetchYamlData{
				Hooks: &ketchv1.KetchYamlHooks{
					Restart: ketchv1.KetchYamlRestartHooks{
						Before: []string{`echo "before"`},
						After:  []string{`echo "after"`},
					},
				},
				Kubernetes: &ketchv1.KetchYamlKubernetesConfig{
					Processes: map[string]ketchv1.KetchYamlProcessConfig{
						"web": {
							Ports: []ketchv1.KetchYamlProcessPortConfig{
								{Name: "web", Protocol: "TCP", Port: 8080, TargetPort: 5000},
								{Name: "socket-port", Protocol: "TCP", Port: 4000},
							},
						},
						"worker": {Ports: []ketchv1.KetchYamlProcessPortConfig{}},
					},
				},
			},
		},
		{
			name: "ketch.yaml contains invalid fields",
			opts: appDeployOptions{
				strictKetchYamlDecoding: true,
				ketchYamlFileName:       "./testdata/invalid-ketch.yaml",
			},
			wantErr:        true,
			wantErrMessage: `error unmarshaling JSON: while decoding JSON: json: unknown field "invalidField"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.opts.KetchYaml()
			if (err != nil) != tt.wantErr {
				t.Errorf("KetchYaml() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && err.Error() != tt.wantErrMessage {
				t.Errorf("KetchYaml() error = %v, wantErr %v", err.Error(), tt.wantErrMessage)
				return
			}
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("KetchYaml() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
