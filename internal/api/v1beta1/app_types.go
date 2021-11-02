/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ShipaCloudDomain     = "shipa.cloud"
	DefaultNumberOfUnits = 1
)

// Env represents an environment variable present in an application.
type Env struct {
	// +kubebuilder:validation:MinLength=1
	// Name of the environment variable. Must be a C_IDENTIFIER.
	Name string `json:"name"`

	// Value of the environment variable.
	Value string `json:"value"`
}

// Label represents an environment variable present in an application.
type Label struct {
	// +kubebuilder:validation:MinLength=1
	// Name of the label.
	Name string `json:"name"`

	// Value of the label.
	Value string `json:"value"`
}

// CnameList is a list of an app's CNAMEs.
type CnameList []Cname

// Cname represents a DNS record and whether the record use TLS.
type Cname struct {
	Name   string `json:"name"`
	Secure bool   `json:"secure"`
	// SecretName if provided must contain an SSL certificate that will be used to serve this cname.
	// Currently, the secret must be in the framework's namespace.
	SecretName string `json:"secretName,omitempty"`
}

// RoutingSettings contains a weight of the current deployment used to route incoming traffic.
// If an application has two deployments with corresponding weights of 30 and 70,
// then 3 of 10 incoming requests will be sent to the first deployment (approximately).
type RoutingSettings struct {
	Weight uint8 `json:"weight"`
}

// ProcessSpec is a specification of the desired behavior of a process.
type ProcessSpec struct {
	// +kubebuilder:validation:MinLength=1
	// Name of the process.
	Name string `json:"name"`

	// Units is a number of replicas of the process.
	Units *int `json:"units,omitempty"`

	// Env is a list of environment variables to set in pods created for the process.
	Env []Env `json:"env,omitempty"`

	// Commands executed on startup.
	Cmd []string `json:"cmd"`

	Resources *v1.ResourceRequirements `json:"resources,omitempty"`

	Volumes []v1.Volume `json:"volumes,omitempty"`

	VolumeMounts []v1.VolumeMount `json:"volumeMounts,omitempty"`

	// Security options the process should run with.
	SecurityContext *v1.SecurityContext `json:"securityContext,omitempty"`
}

type DeploymentVersion int

func (v DeploymentVersion) String() string {
	return fmt.Sprintf("%d", v)
}

type AppDeploymentSpec struct {
	// ImagePullSecrets contains a list of secrets to pull the image of this deployment.
	// If this list is defined, app.Spec.DockerRegistrySpec is not used.
	ImagePullSecrets []v1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	Image            string                    `json:"image"`
	Version          DeploymentVersion         `json:"version"`
	Processes        []ProcessSpec             `json:"processes,omitempty"`
	KetchYaml        *KetchYamlData            `json:"ketchYaml,omitempty"`
	Labels           []Label                   `json:"labels,omitempty"`
	RoutingSettings  RoutingSettings           `json:"routingSettings,omitempty"`
	ExposedPorts     []ExposedPort             `json:"exposedPorts,omitempty"`
}

// IngressSpec configures entrypoints to access an application.
type IngressSpec struct {

	// GenerateDefaultCname if set the application will have a default cname <app-name>.<ServiceEndpoint>.shipa.cloud.
	GenerateDefaultCname bool `json:"generateDefaultCname"`

	// Cnames is a list of additional cnames.
	Cnames CnameList `json:"cnames,omitempty"`
}

// DockerRegistrySpec contains docker registry configuration of an application.
type DockerRegistrySpec struct {

	// SecretName is added to the "imagePullSecrets" list of each application pod.
	SecretName string `json:"secretName,omitempty"`
}

// AppPhase is a label for the condition of an application at the current time.
type AppPhase string

const (
	// AppCreated means the app has been accepted by the system, but has not been started.
	AppCreated AppPhase = "Created"

	// AppError means the app CRD is broken in some way and ketch controller can't render and install a new helm chart.
	AppError AppPhase = "Error"

	// AppRunning means that ketch controller has rendered a helm chart of the application and installed it to a cluster.
	AppRunning AppPhase = "Running"
)

// AppStatus represents information about the status of an application.
type AppStatus struct {

	// Conditions of App resource.
	Conditions []Condition `json:"conditions,omitempty"`

	Framework *v1.ObjectReference `json:"framework,omitempty"`
}

// CanarySpec represents configuration for a canary deployment.
type CanarySpec struct {
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Steps int `json:"steps,omitempty"`
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	StepWeight      uint8         `json:"stepWeight,omitempty"`
	StepTimeInteval time.Duration `json:"stepTimeInterval,omitempty"`
	// NextScheduledTime holds time of the next step.
	NextScheduledTime *metav1.Time `json:"nextScheduledTime,omitempty"`
	// CurrentStep is the count for current step for a canary deployment.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	CurrentStep int `json:"currentStep,omitempty"`
	// Active shows if canary deployment is active for this application.
	Active bool `json:"active,omitempty"`
	// Started holds time when canary started
	Started *metav1.Time `json:"started,omitempty"`
	// Target map of processes and target units value
	Target map[string]uint16 `json:"target,omitempty"`
}

// AppSpec defines the desired state of App.
type AppSpec struct {
	Version *string `json:"version,omitempty"`

	// ID is an additional unique identifier of this application besides the app's name if needed.
	// Ketch internally doesn't rely on this field, so it can be anything useful for a user.
	// Ketch uses either this ID or the app name and adds "app=<ID or name>" label to all pods.
	// ID is preferred and used if set, otherwise the label will be "app=<app-name>".
	// Thus, istio time series will have "destination_app=<ID or name>" label.
	ID string `json:"id,omitempty"`

	// +kubebuilder:validation:MaxLength=140
	Description string `json:"description,omitempty"`

	// Canary contains a configuration which will be required for canary deployments.
	Canary CanarySpec `json:"canary,omitempty"`

	// Deployments is a list of running deployments.
	Deployments []AppDeploymentSpec `json:"deployments"`

	// DeploymentsCount is incremented every time a new deployment is added to Deployments and used as a version for new deployments.
	DeploymentsCount int `json:"deploymentsCount,omitempty"`

	// List of environment variables of the application.
	Env []Env `json:"env,omitempty"`

	// Framework is a name of a Framework used to run the application.
	// +kubebuilder:validation:MinLength=1
	Framework string `json:"framework"`

	// Ingress contains configuration of entrypoints to access the application.
	Ingress IngressSpec `json:"ingress"`

	// DockerRegistry contains docker registry configuration of the application.
	DockerRegistry DockerRegistrySpec `json:"dockerRegistry,omitempty"`

	// Builder is the name of the builder used to build source code.
	Builder string `json:"builder,omitempty"`

	// BuildPacks is a list of build packs to use when building from source.
	BuildPacks []string `json:"buildPacks,omitempty"`

	// Labels is a list of labels that will be applied to Services/Deployments.
	Labels []MetadataItem `json:"labels,omitempty"`

	// Annotations is a list of annotations that will be applied to Services/Deployments/Gateways.
	Annotations []MetadataItem `json:"annotations,omitempty"`

	// ServiceAccountName specifies a service account name to be used for this application.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

// MetadataItem represent a request to add label/annotations to processes
type MetadataItem struct {
	Target            Target            `json:"target,omitempty"`
	Apply             map[string]string `json:"apply,omitempty"`
	DeploymentVersion int               `json:"deploymentVersion,omitempty"`
	ProcessName       string            `json:"processName,omitempty"`
}

type Target struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Framework",type=string,JSONPath=`.spec.framework`
// +kubebuilder:printcolumn:name="Description",type=string,JSONPath=`.spec.description`

// App is the Schema for the apps API.
type App struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppSpec   `json:"spec,omitempty"`
	Status AppStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AppList contains a list of App.
type AppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []App `json:"items"`
}

func (s *AppDeploymentSpec) setUnits(process string, units int) error {
	for i, processSpec := range s.Processes {
		if processSpec.Name == process {
			s.Processes[i].Units = &units
			return nil
		}
	}
	return ErrProcessNotFound
}

func (s *AppDeploymentSpec) setUnitsForAllProcess(units int) {
	for i := range s.Processes {
		s.Processes[i].Units = &units
	}
}

// SetUnits set quantity of units of the specified processes.
func (app *App) SetUnits(selector Selector, units int) error {
	deploymentFound := false
	for _, deploymentSpec := range app.Spec.Deployments {
		if selector.DeploymentVersion != nil && *selector.DeploymentVersion != deploymentSpec.Version {
			continue
		}
		if selector.Process != nil {
			if err := deploymentSpec.setUnits(*selector.Process, units); err != nil {
				return err
			}
		} else {
			deploymentSpec.setUnitsForAllProcess(units)
		}
		deploymentFound = true
	}
	if selector.DeploymentVersion != nil && !deploymentFound {
		return ErrDeploymentNotFound
	}
	return nil
}

// SetEnvs extends the current list of environment variables with the provided list.
// If the current list has an env variable from the provided list, the env variable will be updated with a new value.
func (app *App) SetEnvs(envs []Env) {
	names := make(map[string]Env, len(envs))
	for _, env := range envs {
		names[env.Name] = env
	}
	newEnvs := make([]Env, 0, len(envs))
	for _, env := range app.Spec.Env {
		if newEnv, hasNewValue := names[env.Name]; hasNewValue {
			newEnvs = append(newEnvs, newEnv)
			delete(names, env.Name)
			continue
		}
		newEnvs = append(newEnvs, env)
	}
	for _, env := range names {
		newEnvs = append(newEnvs, env)
	}
	app.Spec.Env = newEnvs
}

// Envs returns values of the asked env variables.
func (app *App) Envs(names []string) map[string]string {
	namesMap := make(map[string]struct{}, len(names))
	for _, name := range names {
		namesMap[name] = struct{}{}
	}

	envs := make(map[string]string)
	for _, env := range app.Spec.Env {
		if len(names) == 0 {
			envs[env.Name] = env.Value
			continue
		}
		if _, ok := namesMap[env.Name]; ok {
			envs[env.Name] = env.Value
		}
	}
	return envs
}

// UnsetEnvs unsets environment values.
func (app *App) UnsetEnvs(envs []string) {
	names := make(map[string]struct{}, len(envs))
	for _, name := range envs {
		names[name] = struct{}{}
	}
	var newEnvs []Env
	for _, env := range app.Spec.Env {
		if _, remove := names[env.Name]; !remove {
			newEnvs = append(newEnvs, env)
		}
	}
	app.Spec.Env = newEnvs
}

// Stop stops processes specified by the selector.
func (app *App) Stop(selector Selector) error {
	return app.SetUnits(selector, 0)
}

// Start starts processes specified by the selector.
// We start a process by setting its unit quantity to 1.
// If a process has running units, nothing will be changed.
func (app *App) Start(selector Selector) error {
	deploymentFound := false
	units := 1
	for _, deploymentSpec := range app.Spec.Deployments {
		if selector.DeploymentVersion != nil && *selector.DeploymentVersion != deploymentSpec.Version {
			continue
		}
		if selector.Process != nil {
			for i, processSpec := range deploymentSpec.Processes {
				if processSpec.Name == *selector.Process && (processSpec.Units == nil || *processSpec.Units == 0) {
					deploymentSpec.Processes[i].Units = &units
				}
			}
		} else {
			for i, processSpec := range deploymentSpec.Processes {
				if processSpec.Units != nil && *processSpec.Units > 1 {
					continue
				}
				deploymentSpec.Processes[i].Units = &units
			}
		}
		deploymentFound = true
	}
	if selector.DeploymentVersion != nil && !deploymentFound {
		return ErrDeploymentNotFound
	}
	return nil
}

// CNames returns all CNAMEs to access the application including a default cname.
func (app *App) CNames(framework *Framework) []string {
	cnames := []string{}
	defaultCname := app.DefaultCname(framework)
	if defaultCname != nil {
		cnames = append(cnames, fmt.Sprintf("http://%s", *defaultCname))
	}
	for _, cname := range app.Spec.Ingress.Cnames {
		scheme := "http"
		if cname.Secure {
			scheme = "https"
		}
		cnames = append(cnames, fmt.Sprintf("%s://%s", scheme, cname.Name))
	}
	return cnames
}

// DefaultCname returns a default cname to access the application.
// A default cname uses the following format: <app name>.<Framework's ServiceEndpoint>.shipa.cloud.
func (app *App) DefaultCname(framework *Framework) *string {
	if framework == nil {
		return nil
	}
	if !app.Spec.Ingress.GenerateDefaultCname {
		return nil
	}
	if len(framework.Spec.IngressController.ServiceEndpoint) == 0 {
		return nil
	}
	url := fmt.Sprintf("%s.%s.%s", app.Name, framework.Spec.IngressController.ServiceEndpoint, ShipaCloudDomain)
	return &url
}

// Units returns a total number units.
func (app *App) Units() int {
	units := 0
	for _, deploymentSpec := range app.Spec.Deployments {
		for _, processSpec := range deploymentSpec.Processes {
			if processSpec.Units == nil {
				units += 1
			} else {
				units += *processSpec.Units
			}
		}
	}
	return units
}

// ExposedPorts returns ports exposed by an image of each deployment.
func (app *App) ExposedPorts() map[DeploymentVersion][]ExposedPort {
	ports := make(map[DeploymentVersion][]ExposedPort, len(app.Spec.Deployments))
	for _, deployment := range app.Spec.Deployments {
		ports[deployment.Version] = deployment.ExposedPorts
	}
	return ports
}

// SetCondition sets Status and message fields of the given type of condition to the provided values.
func (app *App) SetCondition(t ConditionType, status v1.ConditionStatus, message string, time metav1.Time) {
	c := Condition{
		Type:               t,
		Status:             status,
		LastTransitionTime: &time,
		Message:            message,
	}
	for i, cond := range app.Status.Conditions {
		if cond.Type == t {
			if cond.Status == c.Status && cond.Message == c.Message {
				return
			}
			app.Status.Conditions[i] = c
			return
		}
	}
	app.Status.Conditions = append(app.Status.Conditions, c)
}

// Phase return a simple, high-level summary of where the application is in its lifecycle.
func (app *App) Phase() AppPhase {
	for _, cond := range app.Status.Conditions {
		if cond.Status == v1.ConditionFalse {
			return AppError
		}
	}
	if app.Units() == 0 {
		return AppCreated
	}
	return AppRunning
}

// Condition looks for a condition with the provided type in the condition list and returns it.
func (s AppStatus) Condition(t ConditionType) *Condition {
	for _, c := range s.Conditions {
		if c.Type == t {
			return &c
		}
	}
	return nil
}

// getUpdatedUnits(weight=75, target=4) -> (3 units in the source deploy, 1 unit in the target deployment)
func getUpdatedUnits(weight uint8, targetUnits uint16) (int, int) {
	if weight > 100 {
		weight = 100
	}
	// get the split of p1's units and p2's units
	unitSplit := (float64(weight) / 100) * float64(targetUnits)
	// we want an integer so take the floor of the float and subtract that total from the target
	destUnits := targetUnits - uint16(math.Floor(unitSplit))

	// edge case, we need to have at least 1 source unit
	if targetUnits == destUnits {
		return 1, int(destUnits)
	}
	// subtract destination's units from target to find source's new units
	sourceUnits := targetUnits - destUnits
	return int(sourceUnits), int(destUnits)
}

// DoCanary checks if canary deployment is needed for an app and gradually increases the traffic weight
// based on the canary parameters provided by the users. Use it in app controller.
func (app *App) DoCanary(now metav1.Time, logger logr.Logger, recorder record.EventRecorder) error {
	if !app.Spec.Canary.Active {
		failEvent := newCanaryEvent(app, CanaryNotActiveEvent, CanaryNotActiveEventDesc)
		recorder.AnnotatedEventf(app, failEvent.Annotations, v1.EventTypeNormal, failEvent.Name, failEvent.Message())
		return nil
	}

	if len(app.Spec.Deployments) <= 1 {
		failEvent := newCanaryEvent(app, CanaryNoDeployments, CanaryNoDeploymentsDesc)
		recorder.AnnotatedEventf(app, failEvent.Annotations, v1.EventTypeWarning, failEvent.Name, failEvent.Message())
		return errors.New("no canary deployment found")
	}

	if app.Spec.Canary.NextScheduledTime == nil {
		failEvent := newCanaryEvent(app, CanaryNoScheduledSteps, CanaryNoScheduledStepsDesc)
		recorder.AnnotatedEventf(app, failEvent.Annotations, v1.EventTypeWarning, failEvent.Name, failEvent.Message())
		return errors.New("canary is active but the next step is not scheduled")
	}

	if app.Spec.Canary.NextScheduledTime.Equal(&now) || app.Spec.Canary.NextScheduledTime.Before(&now) {
		if app.Spec.Canary.CurrentStep == 1 {
			event := newCanaryEvent(app, CanaryStarted, CanaryStartedDesc)
			recorder.AnnotatedEventf(app, event.Annotations, v1.EventTypeNormal, event.Name, event.Message())
		}
		// update traffic weight distributions across deployments
		app.Spec.Deployments[0].RoutingSettings.Weight = app.Spec.Deployments[0].RoutingSettings.Weight - app.Spec.Canary.StepWeight
		app.Spec.Deployments[1].RoutingSettings.Weight = app.Spec.Deployments[1].RoutingSettings.Weight + app.Spec.Canary.StepWeight

		eventStep := newCanaryNextStepEvent(app)
		recorder.AnnotatedEventf(app, eventStep.Event.Annotations, v1.EventTypeNormal, eventStep.Event.Name, eventStep.Message())

		if app.Spec.Canary.Target != nil {
			// scale units based on weight and process target
			for processName, target := range app.Spec.Canary.Target {
				p1Units, p2Units := getUpdatedUnits(app.Spec.Deployments[0].RoutingSettings.Weight, target)
				// might be fine to ignore these errors
				if err := app.Spec.Deployments[0].setUnits(processName, p1Units); err != nil {
					logger.Info("the process: %s is not present in the previous deployment\n", processName)
				}
				if err := app.Spec.Deployments[1].setUnits(processName, p2Units); err != nil {
					logger.Info("the process: %s in not present in the updated deployment\n", processName)
				}

				eventTarget := newCanaryTargetChangeEvent(app, processName, p1Units, p2Units)
				recorder.AnnotatedEventf(app, eventTarget.Event.Annotations, v1.EventTypeNormal, eventTarget.Event.Name, eventTarget.Message())
			}

			// if a process in the updated deployment isn't found in target create 1 unit
			for _, process := range app.Spec.Deployments[1].Processes {
				if _, found := app.Spec.Canary.Target[process.Name]; !found {
					_ = app.Spec.Deployments[1].setUnits(process.Name, 1)
				}
			}
			// for previous deployment, any processes not in target will be terminated by the end of the canary deployment
		}

		// update next scheduled time
		*app.Spec.Canary.NextScheduledTime = metav1.NewTime(app.Spec.Canary.NextScheduledTime.Add(app.Spec.Canary.StepTimeInteval))

		// check if the canary weight is exceeding 100% of traffic
		if app.Spec.Deployments[1].RoutingSettings.Weight >= 100 || app.Spec.Canary.CurrentStep == app.Spec.Canary.Steps {
			// canary is finished, update new deployment to the target values
			for i, process := range app.Spec.Deployments[1].Processes {
				if target, found := app.Spec.Canary.Target[process.Name]; found {
					finalUnits := int(target)
					process.Units = &finalUnits
					app.Spec.Deployments[1].Processes[i] = process
				}
			}

			// we need to set weight of the target deployment to 100
			// because there is a chance that on the last step weight is not equal to 100 (e.g. steps=3, step-weight=33)
			app.Spec.Deployments[1].RoutingSettings.Weight = 100

			app.Spec.Canary.Active = false
			app.Spec.Canary.CurrentStep = app.Spec.Canary.Steps
			app.Spec.Canary.NextScheduledTime = nil

			eventFinished := newCanaryEvent(app, CanaryFinished, CanaryFinishedDesc)
			recorder.AnnotatedEventf(app, eventFinished.Annotations, v1.EventTypeNormal, eventFinished.Name, eventFinished.Message())

			app.Spec.Deployments = []AppDeploymentSpec{app.Spec.Deployments[1]}
		}
		app.Spec.Canary.CurrentStep++
	}

	return nil
}

// DoRollback performs rollback
func (app *App) DoRollback() {
	// we need to rollback all weight to the primary deployment
	app.Spec.Deployments[0].RoutingSettings.Weight = 100
	app.Spec.Deployments[1].RoutingSettings.Weight = 0
	app.Spec.Canary.Active = false
}

// PodState describes the simplified state of a pod in the cluster
type PodState string

const (
	// PodRunning means that pod running on the cluster
	PodRunning PodState = "running"
	// PodDeploying means that pod is creating on the cluster, it is not in running or error state
	PodDeploying PodState = "deploying"
	// PodError means that the pod is not in a healthy state, and action from the user may be needed
	PodError PodState = "error"
	// PodSucceeded means that all containers in the pod have voluntarily terminated
	// with a container exit code of 0, and the system is not going to restart any of these containers.
	PodSucceeded PodState = "succeeded"
)

// Validate validates the key for annotations & labels. See: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/#syntax-and-character-set
// https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/#syntax-and-character-set
func (m *MetadataItem) Validate() error {
	for key := range m.Apply {
		ok, err := regexp.MatchString(`^([A-Za-z].{0,252}\/)?[A-Z0-9a-z][A-Z0-9a-z-_\.]{0,63}$`, key)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("malformed metadata key")
		}
	}
	return nil
}

// IsDeployment returns true if the target is a Deployment.
func (t Target) IsDeployment() bool {
	return t.Kind == "Deployment" && t.APIVersion == "apps/v1"
}

// IsService returns true if the target is a Service.
func (t Target) IsService() bool {
	return t.Kind == "Service" && t.APIVersion == "v1"
}

const (
	CanaryNotActiveEvent     = "CanaryNotActive"
	CanaryNotActiveEventDesc = "error - canary triggered, but not active"

	CanaryNoDeployments     = "CanaryNoDeployments"
	CanaryNoDeploymentsDesc = "error - canary needs more than 1 deployment to run"

	CanaryNoScheduledSteps     = "CanaryNoScheduledSteps"
	CanaryNoScheduledStepsDesc = "error - canary triggered, but no scheduled steps"

	CanaryStarted      = "CanaryStarted"
	CanaryStartedDesc  = "started"
	CanaryFinished     = "CanaryFinished"
	CanaryFinishedDesc = "finished"

	CanaryNextStep       = "CanaryNextStep"
	CanaryNextStepDesc   = "weight change"
	CanaryStepTarget     = "CanaryStepTarget"
	CanaryStepTargetDesc = "units change"

	CanaryAnnotationAppName            = "canary.shipa.io/app-name"
	CanaryAnnotationDevelopmentVersion = "canary.shipa.io/deployment-version"
	CanaryAnnotationEventName          = "canary.shipa.io/event-name"
	CanaryAnnotationDescription        = "canary.shipa.io/description"
	CanaryAnnotationStep               = "canary.shipa.io/step"
	CanaryAnnotationVersionSource      = "canary.shipa.io/version-source"
	CanaryAnnotationVersionDest        = "canary.shipa.io/version-dest"
	CanaryAnnotationWeightSource       = "canary.shipa.io/weight-source"
	CanaryAnnotationWeightDest         = "canary.shipa.io/weight-dest"
	CanaryAnnotationProcessName        = "canary.shipa.io/process-name"
	CanaryAnnotationProcessUnitsSource = "canary.shipa.io/source-process-units"
	CanaryAnnotationProcessUnitsDest   = "canary.shipa.io/dest-process-units"
)

type CanaryEvent struct {
	// AppName represents event for certain app
	AppName string
	// DeploymentVersion represents for which deployment event is associated with
	DeploymentVersion int

	// Name represents canary event name. It is translated into Reason column of kubernetes event
	// values: CanaryStarted, CanaryFinished
	// errored values: CanaryNotActiveEvent, CanaryNoDeployments, CanaryNoScheduledSteps
	Name string
	// Description states what is the outcome of this event
	Description string
	// Annotations contain details on the Canary deployment
	Annotations map[string]string
}

func newCanaryEvent(app *App, event string, desc string) CanaryEvent {
	var version DeploymentVersion
	if len(app.Spec.Deployments) > 0 {
		version = app.Spec.Deployments[len(app.Spec.Deployments)-1].Version
	}

	return CanaryEvent{
		AppName:           app.Name,
		DeploymentVersion: int(version),
		Name:              event,
		Description:       desc,
		Annotations: map[string]string{
			CanaryAnnotationAppName:            app.Name,
			CanaryAnnotationDevelopmentVersion: version.String(),
			CanaryAnnotationEventName:          event,
			CanaryAnnotationDescription:        desc,
		},
	}
}

// Message is message for k8s event
func (c CanaryEvent) Message() string {
	return fmt.Sprintf("%s - Canary for app %s | version %d - %s", c.Name, c.AppName, c.DeploymentVersion, c.Description)
}

// CanaryEventFromAnnotations creates CanaryEvent from given message
func CanaryEventFromAnnotations(annotations map[string]string) (*CanaryEvent, error) {
	event := CanaryEvent{}
	if value, found := annotations[CanaryAnnotationAppName]; found {
		event.AppName = value
	}
	if value, found := annotations[CanaryAnnotationEventName]; found {
		event.Name = value
	}
	if value, found := annotations[CanaryAnnotationDescription]; found {
		event.Description = value
	}
	if value, found := annotations[CanaryAnnotationDevelopmentVersion]; found {
		version, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf(`unable to parse CanaryEvent, err: %v`, err)
		}
		event.DeploymentVersion = version
	}

	return &event, nil
}

type CanaryNextStepEvent struct {
	Event CanaryEvent

	Step          int
	VersionSource int
	VersionDest   int
	WeightSource  uint8
	WeightDest    uint8
}

func newCanaryNextStepEvent(app *App) CanaryNextStepEvent {
	event := CanaryNextStepEvent{
		Step:          app.Spec.Canary.CurrentStep,
		VersionSource: int(app.Spec.Deployments[0].Version),
		VersionDest:   int(app.Spec.Deployments[1].Version),
		WeightSource:  app.Spec.Deployments[0].RoutingSettings.Weight,
		WeightDest:    app.Spec.Deployments[1].RoutingSettings.Weight,
	}
	additionalAnnotations := map[string]string{
		CanaryAnnotationStep:          strconv.Itoa(app.Spec.Canary.CurrentStep),
		CanaryAnnotationVersionSource: app.Spec.Deployments[0].Version.String(),
		CanaryAnnotationVersionDest:   app.Spec.Deployments[1].Version.String(),
		CanaryAnnotationWeightSource:  strconv.Itoa(int(app.Spec.Deployments[0].RoutingSettings.Weight)),
		CanaryAnnotationWeightDest:    strconv.Itoa(int(app.Spec.Deployments[1].RoutingSettings.Weight)),
	}
	base := newCanaryEvent(app, CanaryNextStep, CanaryNextStepDesc)
	for key, value := range additionalAnnotations {
		base.Annotations[key] = value
	}
	event.Event = base
	return event
}

func (c CanaryNextStepEvent) Message() string {
	return fmt.Sprintf("%s: Step: %d | Source version: %d | Dest version: %d | Source weight: %d | Dest weight: %d", c.Event.Message(), c.Step, c.VersionSource, c.VersionDest, c.WeightSource, c.WeightDest)
}

// CanaryNextStepEventFromAnnotations creates CanaryNextStepEvent from given annotations
func CanaryNextStepEventFromAnnotations(annotations map[string]string) (*CanaryNextStepEvent, error) {
	event := CanaryNextStepEvent{}
	base, err := CanaryEventFromAnnotations(annotations)
	if err != nil {
		return nil, err
	}
	event.Event = *base

	if value, found := annotations[CanaryAnnotationStep]; found {
		step, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf(`unable to parse CanaryEvent, err: %v`, err)
		}
		event.Step = step
	}
	if value, found := annotations[CanaryAnnotationVersionSource]; found {
		version, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf(`unable to parse CanaryEvent, err: %v`, err)
		}
		event.VersionSource = version
	}
	if value, found := annotations[CanaryAnnotationVersionDest]; found {
		version, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf(`unable to parse CanaryEvent, err: %v`, err)
		}
		event.VersionDest = version
	}
	if value, found := annotations[CanaryAnnotationWeightSource]; found {
		weight, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf(`unable to parse CanaryEvent, err: %v`, err)
		}
		event.WeightSource = uint8(weight)
	}
	if value, found := annotations[CanaryAnnotationWeightDest]; found {
		weight, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf(`unable to parse CanaryEvent, err: %v`, err)
		}
		event.WeightDest = uint8(weight)
	}

	return &event, nil
}

type CanaryTargetChangeEvent struct {
	Event CanaryEvent

	VersionSource           int
	VersionDest             int
	ProcessName             string
	SourceProcessUnits      int
	DestinationProcessUnits int
}

func newCanaryTargetChangeEvent(app *App, processName string, sourceUnits, destUnits int) CanaryTargetChangeEvent {
	event := CanaryTargetChangeEvent{
		VersionSource:           int(app.Spec.Deployments[0].Version),
		VersionDest:             int(app.Spec.Deployments[1].Version),
		ProcessName:             processName,
		SourceProcessUnits:      sourceUnits,
		DestinationProcessUnits: destUnits,
	}
	additionalAnnotations := map[string]string{
		CanaryAnnotationVersionSource:      app.Spec.Deployments[0].Version.String(),
		CanaryAnnotationVersionDest:        app.Spec.Deployments[1].Version.String(),
		CanaryAnnotationProcessName:        processName,
		CanaryAnnotationProcessUnitsSource: strconv.Itoa(sourceUnits),
		CanaryAnnotationProcessUnitsDest:   strconv.Itoa(destUnits),
	}
	base := newCanaryEvent(app, CanaryStepTarget, CanaryStepTargetDesc)
	for key, value := range additionalAnnotations {
		base.Annotations[key] = value
	}
	event.Event = base
	return event
}

func (c CanaryTargetChangeEvent) Message() string {
	return fmt.Sprintf("%s: Source version: %d | Dest version: %d | Process: %s | Source units: %d | Dest units: %d", c.Event.Message(), c.VersionSource, c.VersionDest, c.ProcessName, c.SourceProcessUnits, c.DestinationProcessUnits)
}

// CanaryTargetChangeEventFromAnnotations creates CanaryTargetChangeEvent from given annotations
func CanaryTargetChangeEventFromAnnotations(annotations map[string]string) (*CanaryTargetChangeEvent, error) {
	event := CanaryTargetChangeEvent{}
	base, err := CanaryEventFromAnnotations(annotations)
	if err != nil {
		return nil, err
	}
	event.Event = *base

	if value, found := annotations[CanaryAnnotationProcessName]; found {
		event.ProcessName = value
	}
	if value, found := annotations[CanaryAnnotationVersionSource]; found {
		version, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf(`unable to parse CanaryEvent, err: %v`, err)
		}
		event.VersionSource = version
	}
	if value, found := annotations[CanaryAnnotationVersionDest]; found {
		version, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf(`unable to parse CanaryEvent, err: %v`, err)
		}
		event.VersionDest = version
	}
	if value, found := annotations[CanaryAnnotationProcessUnitsSource]; found {
		weight, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf(`unable to parse CanaryEvent, err: %v`, err)
		}
		event.SourceProcessUnits = weight
	}
	if value, found := annotations[CanaryAnnotationProcessUnitsDest]; found {
		weight, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf(`unable to parse CanaryEvent, err: %v`, err)
		}
		event.DestinationProcessUnits = weight
	}

	return &event, nil
}

const (
	AppReconcileOutcomeReason = "AppReconcileOutcome"
)

// AppReconcileOutcome handle information about app reconcile
type AppReconcileOutcome struct {
	AppName         string
	DeploymentCount int
}

// String is a Stringer interface implementation
func (r *AppReconcileOutcome) String(err ...error) string {
	if err != nil {
		return fmt.Sprintf(`app %s %d reconcile fail: %v`, r.AppName, r.DeploymentCount, err)
	}
	return fmt.Sprintf(`app %s %d reconcile success`, r.AppName, r.DeploymentCount)
}

// ParseAppReconcileOutcome makes AppReconcileOutcome from the incoming event reason string
func ParseAppReconcileOutcome(in string) (*AppReconcileOutcome, error) {
	rm := AppReconcileOutcome{}
	_, err := fmt.Sscanf(in, `app %s %d reconcile`, &rm.AppName, &rm.DeploymentCount)
	if err != nil {
		return nil, fmt.Errorf(`unable to parse reconcile reason: %v`, err)
	}
	return &rm, nil
}
