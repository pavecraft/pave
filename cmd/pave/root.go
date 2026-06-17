package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

// configFlag holds the --config path shared across subcommands.
var (
	configFlag  string
	verboseFlag bool
)

// newRootCmd builds the root command and wires all subcommands.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "pave",
		Version: version,
		Short:   "Autonomous local-first orchestrator for AI coding CLIs",
		Long: "pave reads a project's feature spec, tracks implementation state, and\n" +
			"drives an AI coding CLI (claude -p / copilot -p) to implement pending\n" +
			"features, pausing and resuming around provider rate limits.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			configureLogging(verboseFlag)
		},
	}

	root.PersistentFlags().StringVar(&configFlag, "config", ".pave/pave.yaml", "path to pave.yaml")
	root.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "enable debug logging")

	root.AddCommand(
		newInitCmd(),
		newVersionCmd(),
		newStatusCmd(),
		newRunCmd(),
		newLimitsCmd(),
		newUICmd(),
		newUpdateCmd(),
	)
	return root
}

// configureLogging installs a process-wide structured logger. With verbose set,
// the level drops to Debug.
func configureLogging(verbose bool) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}
