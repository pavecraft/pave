package main

import (
	"context"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/pavecraft/pave/internal/config"
	"github.com/pavecraft/pave/internal/project"
	"github.com/pavecraft/pave/internal/scanner"
	"github.com/pavecraft/pave/internal/state"
)

func newStatusCmd() *cobra.Command {
	var scan bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show implemented vs. pending features",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd, configFlag, scan)
		},
	}
	cmd.Flags().BoolVar(&scan, "scan", false, "scan the codebase to refine pending features to implemented")
	return cmd
}

// runStatus loads config, parses the spec, reconciles with persisted state, and
// prints a per-feature table with summary counts. When scan is true, the
// codebase is scanned to refine pending features that are already referenced.
func runStatus(cmd *cobra.Command, configPath string, scan bool) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	spec, err := project.ParseFile(cfg.FeaturesFile)
	if err != nil {
		return err
	}

	if scan {
		found, serr := scanner.Scan(cfg.ProjectPath, spec)
		if serr != nil {
			return serr
		}
		spec = scanner.Refine(spec, found)
	}

	st, err := state.New(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer st.Close()

	// Reconcile against the most recent run's persisted feature state, if any.
	prior, err := latestFeatures(ctx, st)
	if err != nil {
		return err
	}
	rows := state.Reconcile(spec, prior, "", time.Now())

	printFeatureTable(cmd, rows)
	return nil
}

// latestFeatures returns the persisted features of the most recent run, or nil
// if there are no runs yet.
func latestFeatures(ctx context.Context, st state.Store) ([]state.FeatureRow, error) {
	runs, err := st.ListRuns(ctx, 1)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, nil
	}
	return st.ListFeatures(ctx, runs[0].ID)
}

func printFeatureTable(cmd *cobra.Command, rows []state.FeatureRow) {
	out := cmd.OutOrStdout()
	counts := map[project.Status]int{}

	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "STATUS\tID\tTITLE")
	for _, r := range rows {
		counts[r.Status]++
		fmt.Fprintf(tw, "%s\t%s\t%s\n", statusMark(r.Status), r.ID, r.Title)
	}
	tw.Flush()

	fmt.Fprintf(out, "\n%d implemented, %d in progress, %d pending, %d failed (of %d)\n",
		counts[project.StatusImplemented],
		counts[project.StatusInProgress],
		counts[project.StatusPending],
		counts[project.StatusFailed],
		len(rows),
	)
}

func statusMark(s project.Status) string {
	switch s {
	case project.StatusImplemented:
		return "[x]"
	case project.StatusInProgress:
		return "[~]"
	case project.StatusFailed:
		return "[!]"
	default:
		return "[ ]"
	}
}
