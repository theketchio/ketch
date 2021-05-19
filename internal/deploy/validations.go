package deploy

import (
	"context"
	"errors"
	"fmt"
	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	kerrs "github.com/shipa-corp/ketch/internal/errors"
	"github.com/shipa-corp/ketch/internal/validation"
	"os"
)

type StatusType int

const (
	MissingValue StatusType = iota
	InvalidValue
)

type Statuser interface {
	Status() StatusType
}

type Error struct {
	Reason  StatusType
	Message string
}

func (e Error) Status() StatusType { return e.Reason }
func (e Error) Error() string      { return e.Message }
func (e Error) String() string     { return e.Message }

func NewMissingError(flag string) error {
	return &Error{
		Reason:  MissingValue,
		Message: fmt.Sprintf("%q missing", flag),
	}
}

func NewInvalidError(flag string) error {
	return &Error{
		Reason:  InvalidValue,
		Message: fmt.Sprintf("%q invalid value", flag),
	}
}

func isMissing(err error) bool {
	if err == nil {
		return false
	}
	var v *Error
	if errors.As(err, &v) {
		if v.Status() == MissingValue {
			return true
		}
	}
	return false
}

func isValid(err error) bool {
	if err != nil {
		var v *Error
		if errors.As(err, &v) {
			if v.Status() == InvalidValue {
				return false
			}
		}
	}

	return true
}

func validateDeploy(cs *ChangeSet, app *ketchv1.App) error {
	if _, err := cs.getImage(); err != nil {
		return err
	}

	_, err := cs.getSteps()
	if !isMissing(err) {
		if !isValid(err) {
			return err
		}
		switch deps := len(app.Spec.Deployments); {
		case deps == 0:
			return fmt.Errorf("canary deployment failed. No primary deployment found for the app")
		case deps >= 2:
			return fmt.Errorf("canary deployment failed. Maximum number of two deployments are currently supported")
		}
		if _, err := cs.getStepInterval(); err != nil {
			return err
		}
	}

	wait, err := cs.getWait()
	if !isMissing(err) {
		if wait {
			if _, err := cs.getTimeout(); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateSourceDeploy(cs *ChangeSet, app *ketchv1.App) error {
	if _, err := cs.getSourceDirectory(); err != nil {
		return err
	}

	_, err := cs.getIncludeDirs()
	if !isMissing(err) && !isValid(err) {
		return err
	}

	return validateDeploy(cs, app)
}

func validateCreateApp(ctx context.Context, client getter, appName string, cs *ChangeSet) error {
	if !validation.ValidateName(appName) {
		return fmt.Errorf("app name %q is not valid. name must start with "+
			"a letter follow by up to 39 lower case numbers letters and dashes",
			appName)
	}

	if _, err := cs.getPool(ctx, client); err != nil {
		return err
	}
	if _, err := cs.getImage(); err != nil {
		return err
	}

	return nil
}

func directoryExists(dir string) error {
	fi, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return fmt.Errorf("%w directory doesn't exist", NewInvalidError(dir))
	}
	if err != nil {
		return kerrs.Wrap(err, "test for directory failed")
	}
	if !fi.IsDir() {
		return fmt.Errorf("%w not a directory", NewInvalidError(dir))
	}
	return nil
}

func assign(err error, f func()) error {
	if isMissing(err) {
		return nil
	}
	if isValid(err) {
		f()
		return nil
	}
	return err
}
