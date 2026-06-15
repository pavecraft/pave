package main

import "fmt"

// errNotImplemented is returned by command stubs that are wired in later phases.
func errNotImplemented(name string) error {
	return fmt.Errorf("%s: not implemented yet", name)
}
