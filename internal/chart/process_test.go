package chart

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

type mockConfigurator struct {
	servicePorts   map[string][]v1.ServicePort
	containerPorts map[string][]v1.ContainerPort
}

func (m mockConfigurator) Probes(port int32) (Probes, error) {
	return Probes{}, nil
}

func (m mockConfigurator) ServicePortsForProcess(process string) []v1.ServicePort {
	return m.servicePorts[process]
}

func (m mockConfigurator) ContainerPortsForProcess(process string) []v1.ContainerPort {
	return m.containerPorts[process]
}

func intRef(i int) *int {
	return &i
}

func boolRef(b bool) *bool {
	return &b
}

func TestNewProcess(t *testing.T) {
	tests := []struct {
		name        string
		processName string
		isRoutable  bool
		options     []processOption
		want        *process
		wantErr     error
	}{
		{
			name:        "valid configuration",
			processName: "web",
			isRoutable:  true,
			options: []processOption{
				withCmd([]string{"gunicorn", "-p", "8080"}),
				withUnits(intRef(5)),
				withLifecycle(&v1.Lifecycle{}),
				withSecurityContext(&v1.SecurityContext{Privileged: boolRef(true)}),
				withPortsAndProbes(
					mockConfigurator{
						servicePorts: map[string][]v1.ServicePort{
							"web": {
								{Name: "web-port", Protocol: "TCP", Port: 9999, TargetPort: intstr.IntOrString{IntVal: 9999}},
							},
						},
						containerPorts: map[string][]v1.ContainerPort{
							"web": {
								{Name: "web-port", ContainerPort: 9999},
							},
						},
					},
				),
			},
			want: &process{
				Name:     "web",
				Cmd:      []string{"gunicorn", "-p", "8080"},
				Units:    5,
				Routable: true,
				ContainerPorts: []v1.ContainerPort{
					{Name: "web-port", ContainerPort: 9999},
				},
				ServicePorts: []v1.ServicePort{
					{Name: "web-port", Protocol: "TCP", Port: 9999, TargetPort: intstr.IntOrString{IntVal: 9999}},
				},
				PublicServicePort: 9999,
				Env: []ketchv1.Env{
					{Name: "port", Value: "9999"},
					{Name: "PORT", Value: "9999"},
					{Name: "PORT_web", Value: "9999"},
				},
				PodExtra: podExtra{
					Lifecycle: &v1.Lifecycle{},
					SecurityContext: &v1.SecurityContext{
						Privileged: boolRef(true),
					},
				},
			},
		},
		{
			name:        "no service port",
			processName: "web",
			isRoutable:  true,
			options: []processOption{
				withPortsAndProbes(
					mockConfigurator{
						servicePorts: map[string][]v1.ServicePort{},
						containerPorts: map[string][]v1.ContainerPort{
							"web": {
								{Name: "web-port", ContainerPort: 9999},
							},
						},
					},
				),
			},
			wantErr: ErrPortsNotFound,
		},
		{
			name:        "no withPortsAndProbes() - no container port",
			processName: "worker",
			isRoutable:  true,
			options:     []processOption{},
			wantErr:     ErrPortsNotFound,
		},
		{
			name:        "no container port",
			processName: "worker",
			isRoutable:  true,
			options: []processOption{
				withPortsAndProbes(
					&mockConfigurator{
						servicePorts: map[string][]v1.ServicePort{
							"worker": {
								{Name: "web-port", Protocol: "TCP", Port: 9999, TargetPort: intstr.IntOrString{IntVal: 9999}},
							},
						},
						containerPorts: map[string][]v1.ContainerPort{
							"worker": {},
						},
					},
				),
			},
			wantErr: ErrPortsNotFound,
		},
		{
			name:        "valid configuration, non routable",
			processName: "web",
			isRoutable:  false,
			options: []processOption{
				withPortsAndProbes(
					&mockConfigurator{
						servicePorts: map[string][]v1.ServicePort{
							"web": {
								{Protocol: "TCP", Port: 3333, TargetPort: intstr.IntOrString{IntVal: 3333}},
								{Protocol: "TCP", Port: 555, TargetPort: intstr.IntOrString{IntVal: 555}},
							},
						},
						containerPorts: map[string][]v1.ContainerPort{
							"web": {
								{ContainerPort: 3333},
								{ContainerPort: 555},
							},
						},
					},
				),
			},
			want: &process{
				Name:              "web",
				Units:             ketchv1.DefaultNumberOfUnits,
				PublicServicePort: 3333,
				ContainerPorts: []v1.ContainerPort{
					{ContainerPort: 3333},
					{ContainerPort: 555},
				},
				ServicePorts: []v1.ServicePort{
					{Protocol: "TCP", Port: 3333, TargetPort: intstr.IntOrString{IntVal: 3333}},
					{Protocol: "TCP", Port: 555, TargetPort: intstr.IntOrString{IntVal: 555}},
				},
				Env: []ketchv1.Env{
					{Name: "PORT_web", Value: "3333,555"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newProcess(tt.processName, tt.isRoutable, tt.options...)
			if err != tt.wantErr {
				t.Errorf("newProcess() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("newProcess mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
