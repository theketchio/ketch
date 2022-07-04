package chart

import (
	"fmt"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
)

// Configurator provides a Pod related configuration based on KetchYamlData and Procfile.
type Configurator struct {
	data         ketchv1.KetchYamlData
	procfile     Procfile
	exposedPorts []ketchv1.ExposedPort
	defaultPort  int
}

// NewConfigurator returns a Configurator instance.
func NewConfigurator(data *ketchv1.KetchYamlData, procfile Procfile, exposedPorts []ketchv1.ExposedPort, defaultPort int) Configurator {
	shipaYaml := ketchv1.KetchYamlData{}
	if data != nil {
		shipaYaml = *data
	}
	return Configurator{
		data:         shipaYaml,
		procfile:     procfile,
		exposedPorts: exposedPorts,
		defaultPort:  defaultPort,
	}
}

// Probes represents a Pod's liveness and readiness probes.
type Probes struct {
	// Periodic probe of container liveness.
	// Container will be restarted if the probe fails.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	Liveness *apiv1.Probe
	// Periodic probe of container service readiness.
	// Container will be removed from service endpoints if the probe fails.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	Readiness *apiv1.Probe
	// StartupProbe indicates that the Pod has successfully initialized.
	// If specified, no other probes are executed until this completes successfully.
	// If this probe fails, the Pod will be restarted, just as if the livenessProbe failed.
	// This can be used to provide different probe parameters at the beginning of a Pod's lifecycle,
	// when it might take a long time to load data or warm a cache, than during steady-state operation.
	// This cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	StartupProbe *apiv1.Probe
}

func (c Configurator) Probes() (Probes, error) {
	var result Probes
	hc := c.data.Healthcheck
	if hc == nil {
		return result, nil
	}

	if hc.ReadinessProbe != nil {
		result.Readiness = hc.ReadinessProbe
	}

	if hc.LivenessProbe != nil {
		result.Liveness = hc.LivenessProbe
	}

	if hc.StartupProbe != nil {
		result.StartupProbe = hc.StartupProbe
	}

	return result, nil
}

func (c Configurator) Lifecycle() *apiv1.Lifecycle {
	if c.data.Hooks == nil {
		return nil
	}
	if len(c.data.Hooks.Restart.After) == 0 {
		return nil
	}
	hookCmds := []string{
		"sh", "-c",
		strings.Join(c.data.Hooks.Restart.After, " && "),
	}
	return &apiv1.Lifecycle{
		PostStart: &apiv1.LifecycleHandler{
			Exec: &apiv1.ExecAction{
				Command: hookCmds,
			},
		},
	}
}

func (c Configurator) ProcessPortConfigs(process string) []ketchv1.KetchYamlProcessPortConfig {
	if c.data.Kubernetes != nil {
		podConfig, ok := c.data.Kubernetes.Processes[process]
		if ok {
			return podConfig.Ports
		}
	}
	portConfigs := make([]ketchv1.KetchYamlProcessPortConfig, 0, len(c.exposedPorts))
	for i, port := range c.exposedPorts {
		config := ketchv1.KetchYamlProcessPortConfig{
			Name:       fmt.Sprintf("%s-%d", defaultHttpPortName, i+1),
			Protocol:   strings.ToUpper(port.Protocol),
			Port:       port.Port,
			TargetPort: port.Port,
		}
		portConfigs = append(portConfigs, config)
	}
	return portConfigs
}

func (c Configurator) ContainerPortsForProcess(process string) []apiv1.ContainerPort {
	ports := c.ProcessPortConfigs(process)
	containerPorts := make([]apiv1.ContainerPort, 0, len(ports))
	for _, port := range ports {
		var portInt int
		if port.TargetPort > 0 {
			portInt = port.TargetPort
		} else if port.Port > 0 {
			portInt = port.Port
		} else {
			portInt = c.defaultPort
		}
		containerPort := apiv1.ContainerPort{
			ContainerPort: int32(portInt),
		}
		containerPorts = append(containerPorts, containerPort)
	}
	return containerPorts
}

func (c Configurator) ServicePortsForProcess(process string) []apiv1.ServicePort {
	portConfigs := c.ProcessPortConfigs(process)
	servicePorts := make([]apiv1.ServicePort, 0, len(portConfigs))
	for i, portConfig := range portConfigs {

		var targetPort intstr.IntOrString
		if portConfig.TargetPort > 0 {
			targetPort = intstr.FromInt(portConfig.TargetPort)
		} else if portConfig.Port > 0 {
			targetPort = intstr.FromInt(portConfig.Port)
		} else {
			targetPort = intstr.FromInt(c.defaultPort)
		}

		var port int32
		if portConfig.Port > 0 {
			port = int32(portConfig.Port)
		} else if portConfig.TargetPort > 0 {
			port = int32(portConfig.TargetPort)
		} else {
			port = int32(c.defaultPort)
		}

		var name string
		if len(portConfig.Name) > 0 {
			name = portConfig.Name
		} else {
			name = fmt.Sprintf("%s-%d", defaultHttpPortName, i+1)
		}

		sp := apiv1.ServicePort{
			Name:       name,
			Port:       port,
			Protocol:   apiv1.Protocol(portConfig.Protocol),
			TargetPort: targetPort,
		}
		servicePorts = append(servicePorts, sp)
	}
	return servicePorts
}
