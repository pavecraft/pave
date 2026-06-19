package main

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update pave to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := exec.Command("bash", "-c",
				"curl -fsSL https://raw.githubusercontent.com/paveforge/pave/main/install.sh | bash")
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		},
	}
}
