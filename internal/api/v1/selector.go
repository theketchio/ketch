package v1beta1

// Selector specifies targets to apply an action.
// If both a process and version are not specified, an action will be applied to all processes of all deployments.
//
// For example,
// Selector{Process: "web"} specifies all "web" processes of all running deployments.
// Selector{Process: "web", DeploymentVersion: 1} specifies "web" process of a deployment with version 1.
type Selector struct {
	// Process if specified an action will be applied to a particular process otherwise to all processes.
	Process *string

	// DeploymentVersion if specified an action will be applied to a particular deployment otherwise to all deployments.
	DeploymentVersion *DeploymentVersion
}

// NewSelector returns a Selector instance.
func NewSelector(deploymentVersion int, processName string) Selector {
	s := Selector{}
	if processName != "" {
		s.Process = &processName
	}

	if deploymentVersion > 0 {
		version := DeploymentVersion(deploymentVersion)
		s.DeploymentVersion = &version
	}
	return s
}
