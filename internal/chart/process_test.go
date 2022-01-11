package chart

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
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
	memorySize := resource.NewQuantity(5*1024*1024*1024, resource.BinarySI)
	cores := resource.NewMilliQuantity(5300, resource.DecimalSI)
	rr := v1.ResourceRequirements{
		Requests: v1.ResourceList{
			v1.ResourceCPU:    *memorySize,
			v1.ResourceMemory: *cores,
		},
		Limits: v1.ResourceList{
			v1.ResourceCPU:    *memorySize,
			v1.ResourceMemory: *cores,
		},
	}
	volumes := []v1.Volume{{
		Name: "test-volume",
		VolumeSource: v1.VolumeSource{
			AWSElasticBlockStore: &v1.AWSElasticBlockStoreVolumeSource{
				VolumeID: "volume-id",
				FSType:   "ext4",
			},
		},
	}}
	volumeMounts := []v1.VolumeMount{{
		MountPath: "/test-ebs",
		Name:      "test-volume",
	}}

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
				withResourceRequirements(&rr),
				withVolumes(volumes),
				withVolumeMounts(volumeMounts),
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
				Lifecycle: &v1.Lifecycle{},
				SecurityContext: &v1.SecurityContext{
					Privileged: boolRef(true),
				},
				ResourceRequirements: &rr,
				Volumes:              volumes,
				VolumeMounts:         volumeMounts,
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

func Test_withAnnotations(t *testing.T) {

	tests := []struct {
		description       string
		annotations       []ketchv1.MetadataItem
		deploymentVersion ketchv1.DeploymentVersion
		expected          *process
		expectedError     error
	}{
		{
			description: "ok - specify exact deploymentVersion and processName",
			annotations: []ketchv1.MetadataItem{
				{
					Target:            ketchv1.Target{Kind: "Deployment", APIVersion: "apps/v1"},
					DeploymentVersion: 1,
					ProcessName:       "web",
					Apply:             map[string]string{"deployment.io/test": "value"},
				},
				{
					Target:            ketchv1.Target{Kind: "Service", APIVersion: "v1"},
					DeploymentVersion: 1,
					ProcessName:       "web",
					Apply:             map[string]string{"service.io/test": "value"},
				},
				{
					Target:            ketchv1.Target{Kind: "Pod", APIVersion: "v1"},
					DeploymentVersion: 1,
					ProcessName:       "web",
					Apply:             map[string]string{"pod.io/test": "value"},
				},
			},
			deploymentVersion: 1,
			expected: &process{
				Name: "web",
				ServiceMetadata: extraMetadata{
					Annotations: map[string]string{"service.io/test": "value"},
				},
				DeploymentMetadata: extraMetadata{
					Annotations: map[string]string{"deployment.io/test": "value"},
				},
				PodMetadata: extraMetadata{
					Annotations: map[string]string{"pod.io/test": "value"},
				},
			},
		},
		{
			description: "ok - any deploymentVersion and processName",
			annotations: []ketchv1.MetadataItem{
				{
					Target:            ketchv1.Target{Kind: "Service", APIVersion: "v1"},
					DeploymentVersion: 1,
					Apply:             map[string]string{"service.io/any-deployment-1": "any-deployment-1-value"},
				},
				{
					Target:      ketchv1.Target{Kind: "Service", APIVersion: "v1"},
					ProcessName: "web",
					Apply:       map[string]string{"service.io/any-process-web": "any-process-web-value"},
				},
				{
					Target:            ketchv1.Target{Kind: "Pod", APIVersion: "v1"},
					DeploymentVersion: 1,
					Apply:             map[string]string{"pod.io/any-deployment-1": "any-deployment-1-value"},
				},
				{
					Target:      ketchv1.Target{Kind: "Pod", APIVersion: "v1"},
					ProcessName: "web",
					Apply:       map[string]string{"pod.io/any-process-web": "any-process-web-value"},
				},
				{
					Target:            ketchv1.Target{Kind: "Deployment", APIVersion: "apps/v1"},
					DeploymentVersion: 1,
					Apply:             map[string]string{"deployment.io/any-deployment-1": "any-deployment-1-value"},
				},
				{
					Target:      ketchv1.Target{Kind: "Deployment", APIVersion: "apps/v1"},
					ProcessName: "web",
					Apply:       map[string]string{"deployment.io/any-process-web": "any-process-web-value"},
				},
			},
			deploymentVersion: 1,
			expected: &process{
				Name: "web",
				ServiceMetadata: extraMetadata{
					Annotations: map[string]string{
						"service.io/any-deployment-1": "any-deployment-1-value",
						"service.io/any-process-web":  "any-process-web-value",
					},
				},
				DeploymentMetadata: extraMetadata{
					Annotations: map[string]string{
						"deployment.io/any-deployment-1": "any-deployment-1-value",
						"deployment.io/any-process-web":  "any-process-web-value",
					},
				},
				PodMetadata: extraMetadata{
					Annotations: map[string]string{
						"pod.io/any-deployment-1": "any-deployment-1-value",
						"pod.io/any-process-web":  "any-process-web-value",
					},
				},
			},
		},
		{
			description: "error - malformed annotations",
			annotations: []ketchv1.MetadataItem{
				{
					Target:            ketchv1.Target{Kind: "Deployment", APIVersion: "apps/v1"},
					DeploymentVersion: 1,
					ProcessName:       "web",
					Apply:             map[string]string{"_theketch.io/test": "CANT_START_KEY_WITH_UNDERSCORE"},
				},
			},
			deploymentVersion: 1,
			expectedError:     errors.New("malformed metadata key"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			p := &process{Name: "web"}
			fn := withAnnotations(tt.annotations, tt.deploymentVersion)
			err := fn(p)
			if tt.expectedError != nil {
				require.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.Equal(t, tt.expected, p)
			}
		})
	}
}

func Test_withLabels(t *testing.T) {
	tests := []struct {
		name              string
		labels            []ketchv1.MetadataItem
		deploymentVersion ketchv1.DeploymentVersion
		expected          *process
		wantErr           string
	}{
		{
			name: "error - malformed label",
			labels: []ketchv1.MetadataItem{
				{
					Target:            ketchv1.Target{Kind: "Deployment", APIVersion: "apps/v1"},
					DeploymentVersion: 1,
					ProcessName:       "web",
					Apply:             map[string]string{"_theketch.io/test": "CANT_START_KEY_WITH_UNDERSCORE"},
				},
			},
			deploymentVersion: 1,
			wantErr:           "malformed metadata key",
		},
		{
			name: "all good",
			labels: []ketchv1.MetadataItem{
				{
					Target:            ketchv1.Target{Kind: "Deployment", APIVersion: "apps/v1"},
					DeploymentVersion: 1,
					ProcessName:       "web",
					Apply:             map[string]string{"theketch.io/test": "value"},
				},
				{
					Target:            ketchv1.Target{Kind: "Service", APIVersion: "v1"},
					DeploymentVersion: 1,
					ProcessName:       "web",
					Apply:             map[string]string{"theketch.io/test": "value"},
				},
				{
					Target:            ketchv1.Target{Kind: "Deployment", APIVersion: "apps/v1"},
					DeploymentVersion: 2,
					ProcessName:       "web",
					Apply:             map[string]string{"theketch.io/NON-MATCHING-DEPLOYMENT-VERSION": "value"},
				},
				{
					Target:            ketchv1.Target{Kind: "Deployment", APIVersion: "apps/v1"},
					DeploymentVersion: 1,
					ProcessName:       "SOMETHING_ELSE",
					Apply:             map[string]string{"theketch.io/NON-MATCHING-PROCESS": "value"},
				},
				{
					Target:            ketchv1.Target{Kind: "Deployment", APIVersion: "v2"},
					DeploymentVersion: 1,
					ProcessName:       "web",
					Apply:             map[string]string{"theketch.io/NON-MATCHING-VERSION": "value"},
				},
				{
					Target:            ketchv1.Target{Kind: "NON-KIND", APIVersion: "v1"},
					DeploymentVersion: 1,
					ProcessName:       "web",
					Apply:             map[string]string{"theketch.io/NON-EXISTENT-KIND": "value"},
				},
				{
					Target:            ketchv1.Target{Kind: "Pod", APIVersion: "v1"},
					DeploymentVersion: 1,
					ProcessName:       "web",
					Apply:             map[string]string{"pod.label.io": "pod-label"},
				},
				{
					Target:            ketchv1.Target{Kind: "Pod", APIVersion: "v1"},
					DeploymentVersion: 2,
					ProcessName:       "web",
					Apply:             map[string]string{"theketch.io/NON-MATCHING-DEPLOYMENT-VERSION": "value"},
				},
				{
					Target:            ketchv1.Target{Kind: "Pod", APIVersion: "v1"},
					DeploymentVersion: 1,
					ProcessName:       "SOMETHING_ELSE",
					Apply:             map[string]string{"theketch.io/NON-MATCHING-PROCESS": "value"},
				},
			},
			deploymentVersion: 1,
			expected: &process{
				Name: "web",
				ServiceMetadata: extraMetadata{
					Labels: map[string]string{"theketch.io/test": "value"},
				},
				DeploymentMetadata: extraMetadata{
					Labels: map[string]string{"theketch.io/test": "value"},
				},
				PodMetadata: extraMetadata{
					Labels: map[string]string{"pod.label.io": "pod-label"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := withLabels(tt.labels, tt.deploymentVersion)
			p := &process{Name: "web"}
			err := fn(p)
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.expected, p)
		})
	}
}
