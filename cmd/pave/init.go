package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pavecraft/pave/internal/config"
	"github.com/pavecraft/pave/internal/scaffold"
	"github.com/pavecraft/pave/internal/state"
)

func newInitCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold .pave/pave.yaml and FEATURES.md",
		Long: "Creates .pave/pave.yaml (config + database) and FEATURES.md (feature spec).\n" +
			"Add .pave/ to your .gitignore to keep state and config out of version control;\n" +
			"FEATURES.md is placed in the project root so it stays visible to your team.",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := scaffold.Init(dir)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			report(cmd, ".pave/pave.yaml", res.ConfigPath, res.ConfigCreated)
			report(cmd, "FEATURES.md", res.FeaturesPath, res.FeaturesCreated)

			// Initialize the database so state.db exists immediately and the
			// UI can connect before the first `pave run`.
			if err := initDB(cmd, res.ConfigPath); err != nil {
				return err
			}

			if res.ConfigCreated {
				fmt.Fprintf(out, "\nTip: add .pave/ to your .gitignore to exclude config and state from version control.\n")
				fmt.Fprintf(out, "     echo '.pave/' >> .gitignore\n")
			}
			fmt.Fprintf(out, "\nNext: edit FEATURES.md, then:\n")
			fmt.Fprintf(out, "  pave run --dry-run   # preview which features will be implemented\n")
			fmt.Fprintf(out, "  pave run             # implement them\n")
			return nil
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "directory to scaffold into")
	return cmd
}

// initDB opens (and immediately closes) the store so that migrations run and
// the database file is created on disk. Errors are non-fatal: the user can
// still run `pave run` which will create the DB then.
func initDB(cmd *cobra.Command, configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	st, err := state.New(context.Background(), cfg.Database)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "warning: could not initialize database: %v\n", err)
		return nil
	}
	st.Close()
	fmt.Fprintf(cmd.OutOrStdout(), "created  .pave/state.db\n")
	return nil
}

func report(cmd *cobra.Command, name, path string, created bool) {
	if created {
		fmt.Fprintf(cmd.OutOrStdout(), "created %s (%s)\n", name, path)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "skipped %s (already exists: %s)\n", name, path)
	}
}
