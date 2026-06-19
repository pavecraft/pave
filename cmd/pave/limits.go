package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/paveforge/pave/internal/config"
	"github.com/paveforge/pave/internal/state"
)

func newLimitsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "limits",
		Short: "Report current rate-limit status and next reset",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLimits(cmd, configFlag)
		},
	}
}

func runLimits(cmd *cobra.Command, configPath string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	st, err := state.New(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer st.Close()

	providers := []string{cfg.Provider}
	if cfg.FallbackProvider != "" {
		providers = append(providers, cfg.FallbackProvider)
	}

	out := cmd.OutOrStdout()
	for _, p := range providers {
		w, err := st.GetLimiterWindow(ctx, p)
		if err != nil {
			var nf state.ErrNotFound
			if errors.As(err, &nf) {
				fmt.Fprintf(out, "%s: no limit recorded — clear\n", p)
				continue
			}
			return err
		}
		printWindow(out, p, w)
	}
	return nil
}

func printWindow(out interface{ Write([]byte) (int, error) }, provider string, w state.LimiterWindow) {
	now := time.Now()
	if w.ResetAt == nil || !w.ResetAt.After(now) {
		fmt.Fprintf(out, "%s: clear (last limited %s, reason %q)\n",
			provider, w.LimitedAt.Local().Format(time.RFC822), w.Reason)
		return
	}
	remaining := w.ResetAt.Sub(now).Round(time.Second)
	fmt.Fprintf(out, "%s: LIMITED — resets at %s (in %s), reason %q\n",
		provider, w.ResetAt.Local().Format(time.RFC822), remaining, w.Reason)
}
