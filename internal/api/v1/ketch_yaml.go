package v1

import v1 "k8s.io/api/core/v1"

// KetchYamlData describes certain aspects of the application deployment being deployed.
type KetchYamlData struct {

	// Hooks allow to run commands during different stages of the application deployment.
	Hooks *KetchYamlHooks `json:"hooks,omitempty"`

	// Healthcheck describes readiness and liveness probes of the application deployment.
	Healthcheck *KetchYamlHealthcheck `json:"healthcheck,omitempty"`

	// Kubernetes contains specific configurations for Kubernetes.
	Kubernetes *KetchYamlKubernetesConfig `json:"kubernetes,omitempty"`
}

// KetchYamlHooks describes commands to run during different stages of the application deployment.
type KetchYamlHooks struct {

	// Restart describes commands to run during different stages of the application deployment.
	Restart KetchYamlRestartHooks `json:"restart,omitempty"`
}

// KetchYamlRestartHooks describes commands to run during different stages of the application deployment.
type KetchYamlRestartHooks struct {

	// Before contains commands that are executed before a unit is restarted. Commands listed in this hook run once per unit.
	Before []string `json:"before,omitempty" bson:",omitempty"`

	// Before contains commands that are executed after a unit is restarted. Commands listed in this hook run once per unit.
	After []string `json:"after,omitempty" bson:",omitempty"`
}

// KetchYamlHealthcheck describes readiness and liveness probes of the application deployment.
type KetchYamlHealthcheck struct {
	// Periodic probe of container liveness.
	// Container will be restarted if the probe fails.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	LivenessProbe *v1.Probe `json:"livenessProbe,omitempty"`
	// Periodic probe of container service readiness.
	// Container will be removed from service endpoints if the probe fails.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	ReadinessProbe *v1.Probe `json:"readinessProbe,omitempty"`
	// StartupProbe indicates that the Pod has successfully initialized.
	// If specified, no other probes are executed until this completes successfully.
	// If this probe fails, the Pod will be restarted, just as if the livenessProbe failed.
	// This can be used to provide different probe parameters at the beginning of a Pod's lifecycle,
	// when it might take a long time to load data or warm a cache, than during steady-state operation.
	// This cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	StartupProbe *v1.Probe `json:"startupProbe,omitempty"`
}

// KetchYamlKubernetesConfig contains specific configurations for Kubernetes.
type KetchYamlKubernetesConfig struct {

	// Processes configure which ports are exposed on each process of the application deployment.
	Processes map[string]KetchYamlProcessConfig `json:"processes,omitempty"`
}

// KetchYamlKubernetesConfig contains specific configurations of a process.
type KetchYamlProcessConfig struct {
	Ports []KetchYamlProcessPortConfig `json:"ports,omitempty"`
}

// KetchYamlKubernetesConfig contains configuration of an exposed port.
type KetchYamlProcessPortConfig struct {
	// Name is a descriptive name for the port. This field is optional.
	Name string `json:"name,omitempty"`

	// Protocol defines the port protocol. The accepted values are TCP and UDP.
	Protocol string `json:"protocol,omitempty"`

	// Port is the port that will be exposed on a Kubernetes service. If omitted, the target_port value is used.
	Port int `json:"port,omitempty"`

	// TargetPort is the port that the process is listening on. If omitted, the port value is used.
	TargetPort int `json:"target_port,omitempty"`
}
