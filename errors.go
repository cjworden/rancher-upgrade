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

type serviceMapError struct {
	Service string
}

func (e *actionAvailableError) Error() string {
	return fmt.Sprintf("Action %s is not available on service %s", e.Action, e.Service)
}

func (e *upgradeError) Error() string {
	return fmt.Sprintf("Error trying to upgrade %s during the %s action.\n\n%e", e.Service, e.Action, e.Err)
}

func (e *serviceMapError) Error() string {
	return fmt.Sprintf("Error getting service %s from the servicemap.", e.Service)
}
