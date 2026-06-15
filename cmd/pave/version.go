package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pavecraft/pave/internal/config"
)

// version is overridden at build time: -ldflags "-X main.version=v1.2.3"
var version = "dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show pave and pave UI versions",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "pave:     %s\n", version)

			cfg, err := config.Load(configFlag)
			if err != nil {
				fmt.Fprintf(out, "pave UI:  (config unavailable: %v)\n", err)
				return nil
			}

			uiVersion := cfg.UI.Version
			if uiVersion == "" {
				uiVersion = version
			}

			info, err := os.Stat(cfg.UI.Path)
			if err != nil || !info.IsDir() {
				fmt.Fprintf(out, "pave UI:  %s (Not installed)\n", uiVersion)
			} else {
				fmt.Fprintf(out, "pave UI:  %s\n", uiVersion)
			}
			return nil
		},
	}
}
