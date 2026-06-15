// Package config loads, validates, and applies defaults to pave.yaml. It is the
// single source of truth for runtime settings.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Driver identifies a state-store backend.
type Driver string

const (
	DriverSQLite   Driver = "sqlite"
	DriverPostgres Driver = "postgres"
	DriverTurso    Driver = "turso"
)

// Database configures the persistence backend.
type Database struct {
	Driver Driver `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

// Limiter configures rate-limit backoff behavior.
type Limiter struct {
	Window         time.Duration `yaml:"window"`
	BackoffInitial time.Duration `yaml:"backoff_initial"`
	BackoffMax     time.Duration `yaml:"backoff_max"`
}

// Config is the parsed and validated contents of pave.yaml.
type Config struct {
	ProjectPath      string        `yaml:"project_path"`
	FeaturesFile     string        `yaml:"features_file"`
	Provider         string        `yaml:"provider"`
	FallbackProvider string        `yaml:"fallback_provider"`
	Model            string        `yaml:"model"`
	TaskTimeout      time.Duration `yaml:"task_timeout"`
	AutoCommit       bool          `yaml:"auto_commit"`
	MaxRetries       int           `yaml:"max_retries"`
	Database         Database      `yaml:"database"`
	Limiter          Limiter       `yaml:"limiter"`
}

// Default returns a Config populated with the documented default values.
func Default() Config {
	return Config{
		ProjectPath:      ".",
		FeaturesFile:     "./FEATURES.md",
		Provider:         "claude",
		FallbackProvider: "",
		Model:            "",
		TaskTimeout:      30 * time.Minute,
		AutoCommit:       false,
		MaxRetries:       1,
		Database: Database{
			Driver: DriverSQLite,
			DSN:    ".pave/state.db",
		},
		Limiter: Limiter{
			Window:         5 * time.Hour,
			BackoffInitial: time.Minute,
			BackoffMax:     5 * time.Hour,
		},
	}
}

// Load reads and parses the config file at path, applies defaults for any unset
// fields, validates the result, and returns it. Environment variables
// PAVE_DSN and PAVE_DRIVER override the database settings when present.
func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config %q: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config %q: %w", path, err)
	}

	cfg.applyDefaults()
	cfg.applyEnvOverrides()
	cfg.resolvePaths()

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validating config %q: %w", path, err)
	}
	return cfg, nil
}

// applyDefaults restores defaults for fields that were explicitly blanked in the
// YAML (e.g. "provider:" with no value). Omitted fields already retain their
// defaults because Load pre-populates the struct with Default() before
// unmarshaling. Note: numeric fields where zero is a valid choice (MaxRetries)
// are intentionally not coerced here, so "max_retries: 0" is honored.
func (c *Config) applyDefaults() {
	d := Default()
	if c.ProjectPath == "" {
		c.ProjectPath = d.ProjectPath
	}
	if c.FeaturesFile == "" {
		c.FeaturesFile = d.FeaturesFile
	}
	if c.Provider == "" {
		c.Provider = d.Provider
	}
	if c.TaskTimeout == 0 {
		c.TaskTimeout = d.TaskTimeout
	}
	if c.Database.Driver == "" {
		c.Database.Driver = d.Database.Driver
	}
	if c.Database.DSN == "" {
		c.Database.DSN = d.Database.DSN
	}
	if c.Limiter.Window == 0 {
		c.Limiter.Window = d.Limiter.Window
	}
	if c.Limiter.BackoffInitial == 0 {
		c.Limiter.BackoffInitial = d.Limiter.BackoffInitial
	}
	if c.Limiter.BackoffMax == 0 {
		c.Limiter.BackoffMax = d.Limiter.BackoffMax
	}
}

// resolvePaths converts relative paths to absolute, always anchored to the
// current working directory (where pave is executed), not to the config file
// location. Absolute paths and non-file DSNs (postgres://, libsql://, :memory:)
// are left untouched.
func (c *Config) resolvePaths() {
	if abs, err := filepath.Abs(c.ProjectPath); err == nil {
		c.ProjectPath = abs
	}
	if c.Database.Driver == DriverSQLite && isRelativePath(c.Database.DSN) {
		if abs, err := filepath.Abs(c.Database.DSN); err == nil {
			c.Database.DSN = abs
		}
	}
}

// isRelativePath returns true when s looks like a file path (not a URL or
// :memory:) and is not already absolute.
func isRelativePath(s string) bool {
	if s == "" || s == ":memory:" {
		return false
	}
	// URLs contain "://" — leave postgres/libsql/turso DSNs alone.
	if filepath.IsAbs(s) {
		return false
	}
	for _, scheme := range []string{"postgres://", "postgresql://", "libsql://", "http://", "https://"} {
		if len(s) >= len(scheme) && s[:len(scheme)] == scheme {
			return false
		}
	}
	return true
}

// applyEnvOverrides applies PAVE_DSN and PAVE_DRIVER if set.
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("PAVE_DSN"); v != "" {
		c.Database.DSN = v
	}
	if v := os.Getenv("PAVE_DRIVER"); v != "" {
		c.Database.Driver = Driver(v)
	}
}

// Validate checks that the config is internally consistent and that required
// values are present and recognized.
func (c *Config) Validate() error {
	if c.Provider == "" {
		return fmt.Errorf("provider must not be empty")
	}
	if c.FeaturesFile == "" {
		return fmt.Errorf("features_file must not be empty")
	}
	switch c.Database.Driver {
	case DriverSQLite, DriverPostgres, DriverTurso:
	default:
		return fmt.Errorf("unknown database driver %q (want sqlite, postgres, or turso)", c.Database.Driver)
	}
	if c.Database.DSN == "" {
		return fmt.Errorf("database.dsn must not be empty")
	}
	if c.TaskTimeout <= 0 {
		return fmt.Errorf("task_timeout must be positive, got %s", c.TaskTimeout)
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("max_retries must not be negative, got %d", c.MaxRetries)
	}
	if c.Limiter.BackoffInitial > c.Limiter.BackoffMax {
		return fmt.Errorf("limiter.backoff_initial (%s) must not exceed backoff_max (%s)",
			c.Limiter.BackoffInitial, c.Limiter.BackoffMax)
	}
	return nil
}
