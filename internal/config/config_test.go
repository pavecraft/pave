package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "pave.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	return path
}

func TestLoadAppliesDefaults(t *testing.T) {
	// Not parallel: mutates process env in other tests.
	path := writeConfig(t, "provider: claude\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.FeaturesFile != "./FEATURES.md" {
		t.Errorf("FeaturesFile = %q, want default", cfg.FeaturesFile)
	}
	if cfg.TaskTimeout != 30*time.Minute {
		t.Errorf("TaskTimeout = %s, want 30m", cfg.TaskTimeout)
	}
	if cfg.Database.Driver != DriverSQLite {
		t.Errorf("Driver = %q, want sqlite", cfg.Database.Driver)
	}
	// DSN is resolved to absolute by resolvePaths; verify it ends with the
	// expected suffix rather than comparing the raw default string.
	if !filepath.IsAbs(cfg.Database.DSN) || !strings.HasSuffix(cfg.Database.DSN, filepath.Join(".pave", "state.db")) {
		t.Errorf("DSN = %q, want absolute path ending in .pave/state.db", cfg.Database.DSN)
	}
	if cfg.MaxRetries != 1 {
		t.Errorf("MaxRetries = %d, want 1", cfg.MaxRetries)
	}
}

func TestLoadFullConfig(t *testing.T) {
	body := `
project_path: ./myproj
features_file: ./SPEC.md
provider: copilot
fallback_provider: claude
model: sonnet
task_timeout: 10m
auto_commit: true
max_retries: 3
database:
  driver: postgres
  dsn: postgres://localhost/pave
limiter:
  window: 2h
  backoff_initial: 30s
  backoff_max: 1h
`
	path := writeConfig(t, body)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Provider != "copilot" || cfg.FallbackProvider != "claude" {
		t.Errorf("providers = %q/%q", cfg.Provider, cfg.FallbackProvider)
	}
	if !cfg.AutoCommit {
		t.Error("AutoCommit = false, want true")
	}
	if cfg.TaskTimeout != 10*time.Minute {
		t.Errorf("TaskTimeout = %s, want 10m", cfg.TaskTimeout)
	}
	if cfg.Database.Driver != DriverPostgres {
		t.Errorf("Driver = %q, want postgres", cfg.Database.Driver)
	}
	if cfg.Limiter.Window != 2*time.Hour {
		t.Errorf("Window = %s, want 2h", cfg.Limiter.Window)
	}
}

func TestLoadHonorsExplicitZeroRetries(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, "provider: claude\nmax_retries: 0\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.MaxRetries != 0 {
		t.Errorf("MaxRetries = %d, want 0 (explicit zero must be honored)", cfg.MaxRetries)
	}
}

func TestLoadDefaultsRetriesWhenOmitted(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, "provider: claude\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.MaxRetries != 1 {
		t.Errorf("MaxRetries = %d, want default 1", cfg.MaxRetries)
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()
	_, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, "provider: [unclosed\n")
	if _, err := Load(path); err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestLoadRejectsBadDriver(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, "database:\n  driver: mysql\n  dsn: x\n")
	if _, err := Load(path); err == nil {
		t.Fatal("expected validation error for bad driver, got nil")
	}
}

func TestEnvOverrides(t *testing.T) {
	path := writeConfig(t, "provider: claude\n")
	t.Setenv("PAVE_DSN", "libsql://example.turso.io")
	t.Setenv("PAVE_DRIVER", "turso")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Database.Driver != DriverTurso {
		t.Errorf("Driver = %q, want turso (from env)", cfg.Database.Driver)
	}
	if cfg.Database.DSN != "libsql://example.turso.io" {
		t.Errorf("DSN = %q, want env override", cfg.Database.DSN)
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{"default ok", func(*Config) {}, false},
		{"empty provider", func(c *Config) { c.Provider = "" }, true},
		{"empty features file", func(c *Config) { c.FeaturesFile = "" }, true},
		{"empty dsn", func(c *Config) { c.Database.DSN = "" }, true},
		{"bad driver", func(c *Config) { c.Database.Driver = "mongo" }, true},
		{"zero timeout", func(c *Config) { c.TaskTimeout = 0 }, true},
		{"negative retries", func(c *Config) { c.MaxRetries = -1 }, true},
		{"backoff inverted", func(c *Config) {
			c.Limiter.BackoffInitial = 2 * time.Hour
			c.Limiter.BackoffMax = time.Hour
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := Default()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
