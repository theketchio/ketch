package chart

import (
	"reflect"
	"testing"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

func TestKubernetesConfigurator_ProcessCmd(t *testing.T) {
	tests := []struct {
		name     string
		data     ketchv1.KetchYamlData
		procfile Procfile
		process  string
		want     []string
		platform string
	}{
		{
			name: "single command without changing working directory",
			procfile: Procfile{
				Processes: map[string][]string{
					"web": {"python web.py"},
				},
			},
			process:  "web",
			want:     []string{"/bin/sh", "-lc", "exec python web.py"},
			platform: "python",
		},
		{
			name: "single command",
			procfile: Procfile{
				Processes: map[string][]string{
					"web": {"python web.py"},
				},
			},
			process:  "web",
			want:     []string{"/bin/sh", "-lc", "exec python web.py"},
			platform: "python",
		},
		{
			name: "single command with hooks",
			data: ketchv1.KetchYamlData{
				Hooks: &ketchv1.KetchYamlHooks{
					Restart: ketchv1.KetchYamlRestartHooks{
						Before: []string{
							"cmd1", "cmd2",
						},
					},
				},
			},
			procfile: Procfile{
				Processes: map[string][]string{
					"web": {"python web.py"},
				},
			},
			process:  "web",
			want:     []string{"/bin/sh", "-lc", "cmd1 && cmd2 && exec python web.py"},
			platform: "python",
		},
		{
			name: "single command with hooks, without changing working directory",
			data: ketchv1.KetchYamlData{
				Hooks: &ketchv1.KetchYamlHooks{
					Restart: ketchv1.KetchYamlRestartHooks{
						Before: []string{
							"cmd1", "cmd2",
						},
					},
				},
			},
			procfile: Procfile{
				Processes: map[string][]string{
					"web": {"python web.py"},
				},
			},
			process:  "web",
			want:     []string{"/bin/sh", "-lc", "cmd1 && cmd2 && exec python web.py"},
			platform: "python",
		},
		{
			name: "many commands",
			procfile: Procfile{
				Processes: map[string][]string{
					"web": {"python", "web.py"},
				},
			},
			process:  "web",
			want:     []string{"/bin/sh", "-lc", "exec $0 \"$@\"", "python", "web.py"},
			platform: "python",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := Configurator{
				data:     tt.data,
				procfile: tt.procfile,
				platform: tt.platform,
			}
			if got := conf.ProcessCmd(tt.process); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ProcessCmd() = %v, want %v", got, tt.want)
			}
		})
	}
}
