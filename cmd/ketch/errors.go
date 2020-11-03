package main

type cliError string

func (e cliError) Error() string { return string(e) }

const (
	ErrInvalidPoolName cliError = "invalid pool name, pool name should have at most 40 " +
		"characters, containing only lower case letters, numbers or dashes, starting with a letter"

	ErrInvalidAppName cliError = "invalid app name, app name should have at most 40 " +
		"characters, containing only lower case letters, numbers or dashes, starting with a letter"
)
