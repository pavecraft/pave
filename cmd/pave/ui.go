package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/xoai/pave/internal/config"
	"github.com/xoai/pave/internal/proc"
)

func newUICmd() *cobra.Command {
	var (
		uiDir string
		port  string
	)
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Launch the local Next.js viewer (reads the same database)",
		Long: "Starts the Next.js dev server in the ui/ directory with the configured\n" +
			"database injected via PAVE_DRIVER/PAVE_DSN, so the viewer reads the same\n" +
			"data pave writes. Requires Node and a one-time `npm install` in ui/.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUI(cmd, configFlag, uiDir, port)
		},
	}
	cmd.Flags().StringVar(&uiDir, "ui-dir", "", "path to the ui/ directory (default: ./ui or $PAVE_UI_DIR)")
	cmd.Flags().StringVar(&port, "port", "3000", "port for the dev server")
	return cmd
}

func runUI(cmd *cobra.Command, configPath, uiDir, port string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	dir, err := resolveUIDir(uiDir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, "node_modules")); err != nil {
		return fmt.Errorf("dependencies not installed; run `npm install` in %s first", dir)
	}

	env, err := uiEnv(cfg)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Starting pave UI on http://localhost:%s (database: %s)\n", port, cfg.Database.Driver)

	p, err := proc.StartIO(ctx, "npm", []string{"run", "dev", "--", "--port", port}, dir, proc.IOOptions{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Env:    env,
	})
	if err != nil {
		return err
	}
	if _, err := p.Wait(); err != nil {
		return err
	}
	return nil
}

// resolveUIDir locates the ui/ directory: the flag, then $PAVE_UI_DIR, then ./ui.
func resolveUIDir(flagDir string) (string, error) {
	candidates := []string{flagDir, os.Getenv("PAVE_UI_DIR"), "ui"}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if info, err := os.Stat(filepath.Join(c, "package.json")); err == nil && !info.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("could not find the ui/ directory; pass --ui-dir or set PAVE_UI_DIR")
}

// uiEnv builds the environment that points the viewer at pave's database. For
// SQLite the DSN is resolved to an absolute path so it works from the ui/ dir.
func uiEnv(cfg config.Config) ([]string, error) {
	dsn := cfg.Database.DSN
	if cfg.Database.Driver == config.DriverSQLite && !filepath.IsAbs(dsn) {
		abs, err := filepath.Abs(dsn)
		if err != nil {
			return nil, fmt.Errorf("resolving sqlite dsn: %w", err)
		}
		dsn = abs
	}
	env := []string{
		"PAVE_DRIVER=" + string(cfg.Database.Driver),
		"PAVE_DSN=" + dsn,
	}
	if tok := os.Getenv("TURSO_AUTH_TOKEN"); tok != "" {
		env = append(env, "TURSO_AUTH_TOKEN="+tok)
	}
	return env, nil
}
