package v1

// Error is a main error type of this package.
type Error string

func (e Error) Error() string { return string(e) }

const (
	// ErrProcessNotFound is returned when an operation can not be completed because there is no such process.
	ErrProcessNotFound Error = "process not found"

	// ErrDeploymentNotFound is returned when an operation can not be completed because there is no such deployment.
	ErrDeploymentNotFound Error = "deployment not found"

	// ErrJobExists
	ErrJobExists Error = "failed to create job because the job already exists"
)
