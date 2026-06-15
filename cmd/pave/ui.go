package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/xoai/pave/internal/config"
	"github.com/xoai/pave/internal/proc"
)

const uiReleaseURL = "https://github.com/pavecraft/pave/releases/download/%s/pave-ui_%s.tar.gz"

func newUICmd() *cobra.Command {
	var (
		uiDir string
		port  int
	)
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Launch the local Next.js viewer (downloads automatically on first run)",
		Long: "Starts the pave UI server. On first run, pave downloads the pre-built UI\n" +
			"for the matching version and opens your browser automatically.\n" +
			"The viewer reads the same database configured in pave.yaml.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUI(cmd, configFlag, uiDir, port)
		},
	}
	cmd.Flags().StringVar(&uiDir, "ui-dir", "", "path to the UI directory (overrides ui.path in pave.yaml)")
	cmd.Flags().IntVarP(&port, "port", "p", 0, "port for the UI server (overrides ui.port in pave.yaml)")
	return cmd
}

func runUI(cmd *cobra.Command, configPath, uiDirFlag string, portFlag int) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	uiDir := cfg.UI.Path
	if uiDirFlag != "" {
		uiDir = uiDirFlag
	}
	port := cfg.UI.Port
	if portFlag != 0 {
		port = portFlag
	}

	uiVersion := cfg.UI.Version
	if uiVersion == "" {
		uiVersion = version
	}
	if info, err := os.Stat(uiDir); err != nil || !info.IsDir() {
		fmt.Fprintf(cmd.OutOrStdout(), "Downloading pave UI %s...\n", uiVersion)
		if err := downloadUI(uiDir, uiVersion); err != nil {
			return fmt.Errorf("downloading UI: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "UI downloaded to %s\n", uiDir)
	}

	env, err := uiEnv(cfg)
	if err != nil {
		return err
	}
	env = append(env, fmt.Sprintf("PORT=%d", port), "HOSTNAME=0.0.0.0")

	fmt.Fprintf(cmd.OutOrStdout(), "Starting pave UI on http://localhost:%d (database: %s)\n", port, cfg.Database.Driver)

	go func() {
		select {
		case <-time.After(2 * time.Second):
			_ = openBrowser(ctx, fmt.Sprintf("http://localhost:%d", port))
		case <-ctx.Done():
		}
	}()

	p, err := proc.StartIO(ctx, "node", []string{"server.js"}, uiDir, proc.IOOptions{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Env:    env,
	})
	if err != nil {
		return fmt.Errorf("starting UI server: %w", err)
	}
	if _, err := p.Wait(); err != nil {
		return err
	}
	return nil
}

// installedUIVersion returns the version string recorded in destDir/VERSION, or
// an empty string if the file is absent or unreadable.
func installedUIVersion(destDir string) string {
	data, err := os.ReadFile(filepath.Join(destDir, "VERSION"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// downloadUI fetches the pre-built standalone bundle for the given version and
// extracts it into destDir.
func downloadUI(destDir, ver string) error {
	url := fmt.Sprintf(uiReleaseURL, ver, ver)
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s from %s", resp.Status, url)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", destDir, err)
	}

	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("reading gzip stream: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		// Guard against path traversal.
		target := filepath.Join(destDir, filepath.Clean("/"+hdr.Name))
		if !filepath.IsAbs(target) {
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}

	return os.WriteFile(filepath.Join(destDir, "VERSION"), []byte(ver), 0o644)
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
