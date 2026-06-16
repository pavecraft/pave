package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/pavecraft/pave/internal/config"
	"github.com/pavecraft/pave/internal/interactive"
	"github.com/pavecraft/pave/internal/limiter"
	"github.com/pavecraft/pave/internal/planner"
	"github.com/pavecraft/pave/internal/project"
	"github.com/pavecraft/pave/internal/provider"
	"github.com/pavecraft/pave/internal/state"
	"github.com/pavecraft/pave/internal/tui"
)

func newRunCmd() *cobra.Command {
	var (
		feature     string
		dryRun      bool
		maxFeatures int
		yes         bool
	)
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Implement pending features via the configured provider",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRun(cmd, configFlag, runOptions{
				feature:     feature,
				dryRun:      dryRun,
				maxFeatures: maxFeatures,
				yes:         yes,
			})
		},
	}
	cmd.Flags().StringVar(&feature, "feature", "", "implement only this feature ID")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan without invoking the provider")
	cmd.Flags().IntVar(&maxFeatures, "max-features", 0, "stop after N features (0 = no limit)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the permission confirmation prompt")
	return cmd
}

type runOptions struct {
	feature     string
	dryRun      bool
	maxFeatures int
	yes         bool
}

func runRun(cmd *cobra.Command, configPath string, opts runOptions) error {
	// Graceful shutdown on SIGINT/SIGTERM: cancel context, which stops the
	// in-flight subprocess and lets the run persist and exit cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	spec, err := project.ParseFile(cfg.FeaturesFile)
	if err != nil {
		return err
	}

	st, err := state.New(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer st.Close()

	prior, err := latestFeatures(ctx, st)
	if err != nil {
		return err
	}

	run := state.Run{
		ID:        uuid.NewString(),
		Project:   cfg.ProjectPath,
		Provider:  cfg.Provider,
		StartedAt: time.Now(),
		Status:    state.RunRunning,
	}
	if err := st.CreateRun(ctx, run); err != nil {
		return err
	}

	rows := state.Reconcile(spec, prior, run.ID, time.Now())
	rows = selectFeatures(rows, opts)
	for _, r := range rows {
		if err := st.UpsertFeature(ctx, r); err != nil {
			return err
		}
	}

	printProviderSummary(cmd, cfg)

	if opts.dryRun {
		printPlan(cmd, rows)
		return finishRun(ctx, st, run, state.RunCompleted)
	}

	if err := confirmRun(cmd, opts.yes); err != nil {
		return err
	}

	prov, err := providerFor(cfg)
	if err != nil {
		return err
	}
	if err := prov.Available(ctx); err != nil {
		return err
	}

	events, err := interactive.Listen(ctx)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	isATTY := term.IsTerminal(int(os.Stdout.Fd()))

	fmt.Fprintln(out, interactive.Hint)
	fmt.Fprintln(out)

	total := 0
	for _, r := range rows {
		if r.Status != project.StatusImplemented {
			total++
		}
	}
	prog := tui.New(out, isATTY, total)

	eng := &planner.Engine{
		Store:    st,
		Provider: prov,
		Limiter:  limiter.New(ctx, st, prov.Name(), cfg.Limiter, nil),
		Cfg:      cfg,
		Events:   events,
		Out:      out,
		Progress: prog,
		// Discard slog output: all events are persisted to the DB via AppendLogLine
		// and slog lines written to stderr would interleave with the TUI spinner.
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	sum, perr := eng.Process(ctx, run, rows)
	prog.Stop()

	finalStatus := state.RunCompleted
	if ctx.Err() != nil {
		finalStatus = state.RunInterrupted
	} else if perr != nil {
		finalStatus = state.RunFailed
	}
	if ferr := finishRun(ctx, st, run, finalStatus); ferr != nil && perr == nil {
		perr = ferr
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\n%d implemented, %d failed, %d skipped\n",
		sum.Implemented, sum.Failed, sum.Skipped)
	return perr
}

// selectFeatures applies the --feature and --max-features bounds.
func selectFeatures(rows []state.FeatureRow, opts runOptions) []state.FeatureRow {
	if opts.feature != "" {
		for _, r := range rows {
			if r.ID == opts.feature {
				return []state.FeatureRow{r}
			}
		}
		return nil
	}
	if opts.maxFeatures > 0 {
		pending := make([]state.FeatureRow, 0, len(rows))
		for _, r := range rows {
			if r.Status != project.StatusImplemented {
				pending = append(pending, r)
				if len(pending) == opts.maxFeatures {
					break
				}
			}
		}
		return pending
	}
	return rows
}

func printProviderSummary(cmd *cobra.Command, cfg config.Config) {
	out := cmd.OutOrStdout()
	model := cfg.Model
	if model == "" {
		model = "default"
	}
	effort := cfg.Effort
	if effort == "" {
		effort = "default"
	}
	fmt.Fprintf(out, "provider: %s · model: %s · effort: %s\n\n", cfg.Provider, model, effort)
}

func printPlan(cmd *cobra.Command, rows []state.FeatureRow) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "Dry run — planned features:")
	for _, r := range rows {
		if r.Status == project.StatusImplemented {
			continue
		}
		fmt.Fprintf(out, "  - %s (%s)\n", r.ID, r.Title)
	}
}

func finishRun(ctx context.Context, st state.Store, run state.Run, status state.RunStatus) error {
	// Use a background context so the final write succeeds even if ctx was
	// cancelled by a signal.
	end := time.Now()
	return st.UpdateRunStatus(context.WithoutCancel(ctx), run.ID, status, &end)
}

// confirmRun shows a one-time explanation of --dangerously-skip-permissions and
// asks the user to confirm before pave invokes the AI provider. Pass -y / --yes
// to skip the prompt in scripts or CI.
func confirmRun(cmd *cobra.Command, yes bool) error {
	if yes {
		return nil
	}
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "pave needs your permission before it starts.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "To implement features autonomously, pave runs the AI provider (e.g. claude)")
	fmt.Fprintln(out, "in headless mode with --dangerously-skip-permissions. This flag tells the")
	fmt.Fprintln(out, "AI to create, edit, and delete files in your project without pausing to ask")
	fmt.Fprintln(out, "for approval on each step. It is required for unattended operation — without")
	fmt.Fprintln(out, "it the AI waits for interactive input and never makes progress.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "What this means for you:")
	fmt.Fprintln(out, "  • The AI may read, write, or delete any file under your project directory.")
	fmt.Fprintln(out, "  • It will NOT commit, push, or run destructive git commands (enforced by")
	fmt.Fprintln(out, "    the prompt pave sends).")
	fmt.Fprintln(out, "  • You can press Q at any time to stop the run cleanly.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Tip: run `pave run --dry-run` first to see which features will be processed.")
	fmt.Fprintln(out, "     Use `pave run -y` to skip this confirmation in the future.")
	fmt.Fprintln(out, "")
	fmt.Fprint(out, "Proceed? [y/N] ")

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return fmt.Errorf("aborted")
	}
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer != "y" && answer != "yes" {
		return fmt.Errorf("aborted by user")
	}
	fmt.Fprintln(out, "")
	return nil
}

// providerFor builds the provider named in config, wrapping it in a fallback if
// a secondary provider is configured.
func providerFor(cfg config.Config) (provider.Provider, error) {
	primary, err := provider.ByName(cfg.Provider)
	if err != nil {
		return nil, err
	}
	if cfg.FallbackProvider == "" {
		return primary, nil
	}
	secondary, err := provider.ByName(cfg.FallbackProvider)
	if err != nil {
		return nil, fmt.Errorf("fallback: %w", err)
	}
	return &provider.Fallback{Primary: primary, Secondary: secondary}, nil
}
