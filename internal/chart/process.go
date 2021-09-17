package chart

import (
	"errors"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

var (
	ErrPortsNotFound = errors.New("routable process should have at least one container port and one service port")
)

type process struct {
	Name              string             `json:"name"`
	Cmd               []string           `json:"cmd"`
	Units             int                `json:"units"`
	Routable          bool               `json:"routable"`
	ContainerPorts    []v1.ContainerPort `json:"containerPorts"`
	ServicePorts      []v1.ServicePort   `json:"servicePorts"`
	PublicServicePort int32              `json:"publicServicePort,omitempty"`
	Env               []ketchv1.Env      `json:"env"`

	PodExtra podExtra `json:"extra"`
}

type podExtra struct {
	SecurityContext      *v1.SecurityContext      `json:"securityContext,omitempty"`
	ResourceRequirements *v1.ResourceRequirements `json:"resourceRequirements,omitempty"`
	NodeSelectorTerms    []v1.NodeSelectorTerm    `json:"nodeSelectorTerms,omitempty"`
	Volumes              []v1.Volume              `json:"volumes,omitempty"`
	VolumeMounts         []v1.VolumeMount         `json:"volumeMounts,omitempty"`
	ReadinessProbe       *v1.Probe                `json:"readinessProbe,omitempty"`
	LivenessProbe        *v1.Probe                `json:"livenessProbe,omitempty"`
	Lifecycle            *v1.Lifecycle            `json:"lifecycle,omitempty"`
	ServiceMetadata      extraMetadata            `json:"serviceMetadata,omitempty"`
	DeploymentMetadata   extraMetadata            `json:"deploymentMetadata,omitempty"`
}

type extraMetadata struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type processOption func(p *process) error

func withUnits(units *int) processOption {
	return func(p *process) error {
		if units != nil {
			p.Units = *units
		}
		return nil
	}
}

// withEnvs configures env variables of a process.
// Additionally, the process will have port-related envs like "PORT". Check out "portEnvVariables" below.
func withEnvs(envs []ketchv1.Env) processOption {
	return func(p *process) error {
		p.Env = envs
		return nil
	}
}

func withCmd(cmd []string) processOption {
	return func(p *process) error {
		p.Cmd = cmd
		return nil
	}
}

// portConfigurator has methods to work with port related configuration of ketch.yaml.
type portConfigurator interface {
	ContainerPortsForProcess(process string) []v1.ContainerPort
	ServicePortsForProcess(process string) []v1.ServicePort
	Probes(port int32) (Probes, error)
}

func withPortsAndProbes(c portConfigurator) processOption {
	return func(p *process) error {
		p.ServicePorts = c.ServicePortsForProcess(p.Name)
		p.ContainerPorts = c.ContainerPortsForProcess(p.Name)
		if len(p.ContainerPorts) == 0 || len(p.ServicePorts) == 0 {
			return nil
		}
		probes, err := c.Probes(p.ContainerPorts[0].ContainerPort)
		if err != nil {
			return err
		}
		p.PublicServicePort = p.ServicePorts[0].Port
		p.PodExtra.LivenessProbe = probes.Liveness
		p.PodExtra.ReadinessProbe = probes.Readiness
		return nil
	}
}

func withSecurityContext(securityContext *v1.SecurityContext) processOption {
	return func(p *process) error {
		p.PodExtra.SecurityContext = securityContext
		return nil
	}
}

func withLifecycle(lc *v1.Lifecycle) processOption {
	return func(p *process) error {
		p.PodExtra.Lifecycle = lc
		return nil
	}
}

func withResourceRequirements(rr *v1.ResourceRequirements) processOption {
	return func(p *process) error {
		p.PodExtra.ResourceRequirements = rr
		return nil
	}
}

func withVolumes(volumes []v1.Volume) processOption {
	return func(p *process) error {
		p.PodExtra.Volumes = volumes
		return nil
	}
}

func withVolumeMounts(vm []v1.VolumeMount) processOption {
	return func(p *process) error {
		p.PodExtra.VolumeMounts = vm
		return nil
	}
}

// withLabels returns a function that populates Kind labels which specify a matching/unspecified deploymentVersion, a matching/unspecified
// processName, and apiVersion v1.
func withLabels(labels []ketchv1.MetadataItem, deploymentVersion ketchv1.DeploymentVersion) processOption {
	return func(p *process) error {
		for _, label := range labels {
			if label.DeploymentVersion > 0 && int(deploymentVersion) != label.DeploymentVersion ||
				label.ProcessName != "" && label.ProcessName != p.Name ||
				label.Target.APIVersion != "v1" {
				continue
			}
			if err := label.Validate(); err != nil {
				return err
			}
			for k, v := range label.Apply {
				switch label.Target.Kind {
				case "Deployment":
					if p.PodExtra.DeploymentMetadata.Labels == nil {
						p.PodExtra.DeploymentMetadata.Labels = make(map[string]string)
					}
					p.PodExtra.DeploymentMetadata.Labels[k] = v
				case "Service":
					if p.PodExtra.ServiceMetadata.Labels == nil {
						p.PodExtra.ServiceMetadata.Labels = make(map[string]string)
					}
					p.PodExtra.ServiceMetadata.Labels[k] = v
				}
			}
		}
		return nil
	}
}

// withAnnotations returns a function that populates Kind annotations which specify a matching/unspecified deploymentVersion, a matching/unspecified
// processName, and apiVersion v1.
func withAnnotations(annotations []ketchv1.MetadataItem, deploymentVersion ketchv1.DeploymentVersion) processOption {
	return func(p *process) error {
		for _, annotation := range annotations {
			if annotation.DeploymentVersion > 0 && int(deploymentVersion) != annotation.DeploymentVersion ||
				annotation.ProcessName != "" && annotation.ProcessName != p.Name ||
				annotation.Target.APIVersion != "v1" {
				continue
			}
			if err := annotation.Validate(); err != nil {
				return err
			}
			for k, v := range annotation.Apply {
				switch annotation.Target.Kind {
				case "Deployment":
					if p.PodExtra.DeploymentMetadata.Annotations == nil {
						p.PodExtra.DeploymentMetadata.Annotations = make(map[string]string)
					}
					p.PodExtra.DeploymentMetadata.Annotations[k] = v
				case "Service":
					if p.PodExtra.ServiceMetadata.Annotations == nil {
						p.PodExtra.ServiceMetadata.Annotations = make(map[string]string)
					}
					p.PodExtra.ServiceMetadata.Annotations[k] = v
				}
			}
		}
		return nil
	}
}

func newProcess(name string, isRoutable bool, opts ...processOption) (*process, error) {
	process := &process{
		Name:     name,
		Units:    ketchv1.DefaultNumberOfUnits,
		Routable: isRoutable,
	}

	for _, opt := range opts {
		if err := opt(process); err != nil {
			return nil, err
		}
	}

	process.Env = append(process.Env, process.portEnvVariables()...)
	if !process.Routable {
		return process, nil
	}
	// only routable process must have configured ports.
	if !process.hasOpenPort() {
		return nil, ErrPortsNotFound
	}
	return process, nil
}

func (p process) hasOpenPort() bool {
	return len(p.ContainerPorts) > 0 && len(p.ServicePorts) > 0
}

func (p process) portEnvVariables() []ketchv1.Env {
	if len(p.ContainerPorts) == 0 {
		return nil
	}
	var envs []ketchv1.Env
	if len(p.ContainerPorts) == 1 {
		portValue := fmt.Sprintf("%d", p.ContainerPorts[0].ContainerPort)
		envs = append(envs, ketchv1.Env{Name: "port", Value: portValue})
		envs = append(envs, ketchv1.Env{Name: "PORT", Value: portValue})
	}
	ports := make([]string, 0, len(p.ContainerPorts))
	for _, port := range p.ContainerPorts {
		ports = append(ports, fmt.Sprintf("%d", port.ContainerPort))
	}
	envs = append(envs, ketchv1.Env{Name: fmt.Sprintf("PORT_%s", p.Name), Value: strings.Join(ports, ",")})
	return envs
}
