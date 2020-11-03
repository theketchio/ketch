package v1beta1

// Error is a main error type of this package.
type Error string

func (e Error) Error() string { return string(e) }

const (
	// ErrProcessNotFound is returned when an operation can not be completed because there is no such process.
	ErrProcessNotFound Error = "process not found"

	// ErrDeploymentNotFound is returned when an operation can not be completed because there is no such deployment.
	ErrDeploymentNotFound Error = "deployment not found"

	// ErrDeletePoolWithRunningApps is returned when a pool can not be deleted because the pool contains running apps.
	ErrDeletePoolWithRunningApps Error = "failed to delete pool because the pool contains running apps"

	// ErrChangeNamespaceWhenAppsRunning is returned when a pool's namespace can not be changed because the pool contains running apps.
	ErrChangeNamespaceWhenAppsRunning Error = "failed to change target namespace because the pool contains running apps"

	// ErrNamespaceIsUsedByAnotherPool is returned when a pool's namespace can not be changed because there is another pool that uses a new namespace.
	ErrNamespaceIsUsedByAnotherPool Error = "failed to change target namespace because the namespace is already used by another pool"

	// ErrDecreaseQuota is returned when a new quota is too small.
	ErrDecreaseQuota Error = "failed to decrease quota because the pool has more running apps than the new quota permits"
)
