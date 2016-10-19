package main

import (
	"fmt"
)

type actionAvailableError struct {
	Action  string
	Service string
}

type upgradeError struct {
	Action  string
	Service string
	Err     error
}

func (e *actionAvailableError) Error() string {
	return fmt.Sprintf("Action %s is not available on service %s", e.Action, e.Service)
}

func (e *upgradeError) Error() string {
	return fmt.Sprintf("Error trying to upgrade %s during the %s action.\n\n%e", e.Service, e.Action, e.Err)
}
