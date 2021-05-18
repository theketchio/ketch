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

type errValidation string

func (e errValidation) Error() string { return string(e) }

const (
	errMissing errValidation = "missing value"
	errInvalid errValidation = "invalid value"
)

func isMissing(err error) bool {
	if err == nil {
		return false
	}
	var v *errValidation
	if errors.As(err, &v) {
		if *v == errMissing {
			return true
		}
	}
	return false
}

func isValid(err error) bool {
	if err != nil {

		var v *errValidation
		if errors.As(err, &v) {
			if *v == errInvalid {
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
		return fmt.Errorf("%w %q doesn't exist", errInvalid, dir)
	}
	if err != nil {
		return kerrs.Wrap(err, "test for directory failed")
	}
	if !fi.IsDir() {
		return fmt.Errorf("%w %q is not a directory", errInvalid, dir)
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
