package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/pavecraft/pave/internal/api"
	"github.com/pavecraft/pave/internal/config"
	"github.com/pavecraft/pave/internal/proc"
	"github.com/pavecraft/pave/internal/state"
	"github.com/pavecraft/pave/internal/uistatic"
)

func newUICmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Launch the pave UI server",
		Long: "Starts the pave UI server backed by a built-in Go HTTP server.\n" +
			"The viewer reads the same database configured in pave.yaml.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUI(cmd, configFlag, port)
		},
	}
	cmd.Flags().IntVarP(&port, "port", "P", 0, "port for the UI server (default 4000)")
	return cmd
}

func runUI(cmd *cobra.Command, configPath string, portFlag int) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	port := cfg.UI.Port
	if portFlag != 0 {
		port = portFlag
	}

	store, err := state.New(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer store.Close()

	files, err := fs.Sub(uistatic.Files, "dist")
	if err != nil {
		return fmt.Errorf("loading UI assets: %w", err)
	}

	handler := api.NewServer(store, files)
	srv := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: handler}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutCtx) //nolint:errcheck
	}()

	addr := fmt.Sprintf("http://localhost:%d", port)
	fmt.Fprintf(cmd.OutOrStdout(), "Starting pave UI on %s\n", addr)
	go func() {
		select {
		case <-time.After(500 * time.Millisecond):
			_ = openBrowser(ctx, addr)
		case <-ctx.Done():
		}
	}()

	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// openBrowser launches the system browser for url using the platform command.
func openBrowser(ctx context.Context, url string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name = "open"
		args = []string{url}
	case "windows":
		name = "cmd"
		args = []string{"/C", "start", url}
	default:
		name = "xdg-open"
		args = []string{url}
	}
	p, err := proc.Start(ctx, name, args, ".")
	if err != nil {
		return err
	}
	_, _ = p.Wait()
	return nil
}
