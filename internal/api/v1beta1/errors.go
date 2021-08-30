package v1beta1

// Error is a main error type of this package.
type Error string

func (e Error) Error() string { return string(e) }

const (
	// ErrProcessNotFound is returned when an operation can not be completed because there is no such process.
	ErrProcessNotFound Error = "process not found"

	// ErrDeploymentNotFound is returned when an operation can not be completed because there is no such deployment.
	ErrDeploymentNotFound Error = "deployment not found"

	// ErrDeleteFrameworkWithRunningApps is returned when a framework can not be deleted because the framework contains running apps.
	ErrDeleteFrameworkWithRunningApps Error = "failed to delete framework because the framework contains running apps"

	// ErrChangeNamespaceWhenAppsRunning is returned when a framework's namespace can not be changed because the framework contains running apps.
	ErrChangeNamespaceWhenAppsRunning Error = "failed to change target namespace because the framework contains running apps"

	// ErrNamespaceIsUsedByAnotherFramework is returned when a framework's namespace can not be changed because there is another framework that uses a new namespace.
	ErrNamespaceIsUsedByAnotherFramework Error = "failed to change target namespace because the namespace is already used by another framework"

	// ErrDecreaseQuota is returned when a new quota is too small.
	ErrDecreaseQuota Error = "failed to decrease quota because the framework has more running apps than the new quota permits"

	// ErrJobExists
	ErrJobExists Error = "failed to create job because the job already exists"
)
