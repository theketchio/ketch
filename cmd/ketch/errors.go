package main

import (
	"errors"
)

type cliError string

func (e cliError) Error() string { return string(e) }

const (
	ErrInvalidAppName cliError = "invalid app name, app name should have at most 40 " +
		"characters, containing only lower case letters, numbers or dashes, starting with a letter"

	ErrInvalidJobName cliError = "invalid job name, job name should have at most 40 " +
		"characters, containing only lower case letters, numbers or dashes, starting with a letter"

	ErrNoEntrypointAndCmd   cliError = "image doesn't have entrypoint and cmd set"
	ErrLogUnknownTimeFormat cliError = "unknown time format"

	ErrClusterIssuerNotFound cliError = "cluster issuer not found"

	ErrClusterIssuerRequired cliError = "secure cnames require app.Ingress.Controller.ClusterIssuer to be set"
)

func unwrappedError(err error) error {
	for {
		if errors.Unwrap(err) == nil {
			return err
		}
		err = errors.Unwrap(err)
	}
}
