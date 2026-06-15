// Command pave is an autonomous, local-first development orchestrator that
// drives an AI coding CLI to implement features from a spec file.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
