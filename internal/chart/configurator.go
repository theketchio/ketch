package chart

import (
	"fmt"
	"net/http"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/pkg/errors"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

// Configurator provides a Pod related configuration based on KetchYamlData and Procfile.
type Configurator struct {
	data         ketchv1.KetchYamlData
	procfile     Procfile
	exposedPorts []ketchv1.ExposedPort
	defaultPort  int
	platform     string
}

// NewConfigurator returns a Configurator instance.
func NewConfigurator(data *ketchv1.KetchYamlData, procfile Procfile, exposedPorts []ketchv1.ExposedPort, defaultPort int, platform string) Configurator {
	shipaYaml := ketchv1.KetchYamlData{}
	if data != nil {
		shipaYaml = *data
	}
	return Configurator{
		data:         shipaYaml,
		procfile:     procfile,
		exposedPorts: exposedPorts,
		defaultPort:  defaultPort,
		platform:     strings.ToLower(platform),
	}
}

// Probes represents a Pod's liveness and readiness probes.
type Probes struct {
	Liveness  *apiv1.Probe
	Readiness *apiv1.Probe
}

func (c Configurator) Probes(port int32) (Probes, error) {
	var result Probes
	hc := c.data.Healthcheck
	if hc == nil || hc.Path == "" {
		return result, nil
	}
	if hc.Scheme == "" {
		hc.Scheme = defaultHealthcheckScheme
	}
	hc.Method = strings.ToUpper(hc.Method)
	if hc.Method == "" {
		hc.Method = http.MethodGet
	}
	if hc.IntervalSeconds == 0 {
		hc.IntervalSeconds = defaultHealthcheckIntervalSeconds
	}
	if hc.TimeoutSeconds == 0 {
		hc.TimeoutSeconds = defaultHealthcheckTimeoutSeconds
	}
	if !hc.UseInRouter {
		url := fmt.Sprintf("%s://localhost:%d/%s", hc.Scheme, port, strings.TrimPrefix(hc.Path, "/"))
		result.Readiness = &apiv1.Probe{
			FailureThreshold: int32(hc.AllowedFailures),
			PeriodSeconds:    int32(3),
			TimeoutSeconds:   int32(hc.TimeoutSeconds),
			Handler: apiv1.Handler{
				Exec: &apiv1.ExecAction{
					Command: []string{
						"sh", "-c",
						fmt.Sprintf(`if [ ! -f /tmp/onetimeprobesuccessful ]; then curl -ksSf -X%[1]s -o /dev/null %[2]s && touch /tmp/onetimeprobesuccessful; fi`,
							hc.Method, url),
					},
				},
			},
		}
		return result, nil
	}
	if hc.Method != http.MethodGet {
		return result, errors.New("healthcheck: only GET method is supported in with use_in_router set")
	}
	if hc.AllowedFailures == 0 {
		hc.AllowedFailures = defaultHealthcheckAllowedFailures
	}
	hc.Scheme = strings.ToUpper(hc.Scheme)
	probe := &apiv1.Probe{
		FailureThreshold: int32(hc.AllowedFailures),
		PeriodSeconds:    int32(hc.IntervalSeconds),
		TimeoutSeconds:   int32(hc.TimeoutSeconds),
		Handler: apiv1.Handler{
			HTTPGet: &apiv1.HTTPGetAction{
				Path:   hc.Path,
				Port:   intstr.FromInt(int(port)),
				Scheme: apiv1.URIScheme(hc.Scheme),
			},
		},
	}
	result.Readiness = probe
	if hc.ForceRestart {
		result.Liveness = probe
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
		PostStart: &apiv1.Handler{
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

func (c Configurator) ProcessCmd(process string) []string {
	// If we are not using an existing Shipa platform return the commands unmodified. In this case
	// the onus is on the image creator to build an image with entry points/commands that k8s understands.
	if c.platform == "" {
		return c.procfile.Processes[process]
	}
	cmd := c.procfile.Processes[process]
	before := ""
	if c.data.Hooks != nil {
		before = strings.Join(c.data.Hooks.Restart.Before, " && ")
		if before != "" {
			before += " && "
		}
	}
	commands := []string{
		"/bin/sh",
		"-lc",
		before,
	}
	if len(cmd) > 1 {
		commands[len(commands)-1] += "exec $0 \"$@\""
		commands = append(commands, cmd...)
	} else {
		commands[len(commands)-1] += "exec " + cmd[0]
	}
	return commands
}
