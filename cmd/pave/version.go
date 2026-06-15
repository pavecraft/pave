package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is overridden at build time: -ldflags "-X main.version=1.2.3"
var version = "dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show pave and pave UI versions",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "pave:     %s\n", version)
			fmt.Fprintf(out, "pave UI:  %s (embedded)\n", version)
			return nil
		},
	}
}
