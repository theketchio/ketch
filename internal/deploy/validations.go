package deploy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
	kerrs "github.com/theketchio/ketch/internal/errors"
	"github.com/theketchio/ketch/internal/validation"
)

type statusType int

const (
	missingValue statusType = iota
	invalidValue
	invalidUsage
)

type statusError struct {
	reason  statusType
	message string
}

func (e statusError) Status() statusType { return e.reason }
func (e statusError) Error() string      { return e.message }
func (e statusError) String() string     { return e.message }

func newMissingError(flag string) error {
	return &statusError{
		reason:  missingValue,
		message: fmt.Sprintf("%q missing", flag),
	}
}

func newInvalidValueError(flag string) error {
	return &statusError{
		reason:  invalidValue,
		message: fmt.Sprintf("%q invalid value", flag),
	}
}

func newInvalidUsageError(flag string) error {
	return &statusError{
		reason:  invalidUsage,
		message: fmt.Sprintf("%q used improperly", flag),
	}
}

func isMissing(err error) bool {
	if err == nil {
		return false
	}
	var v *statusError
	if errors.As(err, &v) {
		if v.Status() == missingValue {
			return true
		}
	}
	return false
}

func isValid(err error) bool {
	if err != nil {
		var v *statusError
		if errors.As(err, &v) {
			if v.Status() == invalidValue || v.Status() == invalidUsage {
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

	_, err = cs.getUnits()
	if !isMissing(err) {
		if !isValid(err) {
			return err
		}
	}

	_, err = cs.getVersion()
	if !isMissing(err) {
		if !isValid(err) {
			return err
		}
	}

	_, err = cs.getProcess()
	if !isMissing(err) {
		if !isValid(err) {
			return err
		}
	}

	// Volume Validations

	_, err = cs.getVolumeName()
	if !isMissing(err) {
		if !isValid(err) {
			return err
		}
	}

	_, err = cs.getVolumes()
	if !isMissing(err) {
		if !isValid(err) {
			return err
		}
	}

	_, err = cs.getVolumeMounts()
	if !isMissing(err) {
		if !isValid(err) {
			return err
		}
	}

	// SecurityContext Validations

	_, err = cs.getRunAsUser()
	if !isMissing(err) {
		if !isValid(err) {
			return err
		}
	}

	_, err = cs.getFSGroup()
	if !isMissing(err) {
		if !isValid(err) {
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

func validateSourceDeploy(cs *ChangeSet) error {
	sourcePath, err := cs.getSourceDirectory()
	if err != nil {
		return err
	}
	stat, err := os.Stat(path.Join(sourcePath, defaultProcFile))
	if err != nil || stat.IsDir() {
		return fmt.Errorf("%q not found in root of source directory", defaultProcFile)
	}
	return nil
}

func validateCreateApp(ctx context.Context, client Client, appName string, cs *ChangeSet) error {
	if !validation.ValidateName(appName) {
		return fmt.Errorf("app name %q is not valid. name must start with "+
			"a letter follow by up to 39 lower case numbers letters and dashes",
			appName)
	}

	if _, err := cs.getNamespace(); err != nil {
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
		return fmt.Errorf("%w directory doesn't exist", newInvalidValueError(dir))
	}
	if err != nil {
		return kerrs.Wrap(err, "test for directory failed")
	}
	if !fi.IsDir() {
		return fmt.Errorf("%w not a directory", newInvalidValueError(dir))
	}
	return nil
}

func assign(err error, f func() error) error {
	if isMissing(err) {
		return nil
	}
	if isValid(err) {
		return f()
	}
	return err
}
