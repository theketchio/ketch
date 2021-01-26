package v1beta1

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

	// Build defines commands that are run as part of the Dockerfile build
	Build []string `json:"build,omitempty"`

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

	// Path defines which path to call in the application. This path is called for each unit. It is the only mandatory field. If not set, the health check is ignored.
	// +kubebuilder:validation:MinLength=1
	Path string `json:"path"`

	// Method defines the method used to make the http request. The default is GET.
	Method string `json:"method,omitempty"`

	// Scheme defines which scheme to use. The defaults is http.
	Scheme string `json:"scheme,omitempty"`

	// Headers defines optional additional header names that can be used for the request. Header names must be capitalized.
	Headers map[string]string `json:"headers,omitempty" bson:",omitempty"`

	// Match is a regular expression to be matched against the request body.
	// If not set, the body won’t be read and only the status code is checked. This regular expression uses Go syntax and runs with a matching \n (s flag).
	Match string `json:"match,omitempty"`

	// If not set, only readiness probe will be created.
	UseInRouter bool `json:"use_in_router,omitempty"`

	// ForceRestart determines whether a unit should be restarted after allowedFailures encounters consecutive healthcheck failures.
	// Sets the liveness probe in the Pod.
	ForceRestart bool `json:"force_restart,omitempty"`

	// AllowedFailures specifies a number of allowed failures before healthcheck considers the application is unhealthy. The defaults is 0.
	AllowedFailures int `json:"allowed_failures,omitempty"`

	// IntervalSeconds is an interval in seconds between each active healthcheck call if use_in_router is set to true. The default is 10 seconds.
	IntervalSeconds int `json:"interval_seconds,omitempty"`

	// TimeoutSeconds is a timeout for each healthcheck call in seconds. The default is 60 seconds.
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
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
