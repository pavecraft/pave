// Package scaffold writes the starter files created by `pave init`.
package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultConfigYAML is the starter pave.yaml content written to .pave/pave.yaml.
const DefaultConfigYAML = `# pave configuration. See README.md for the full reference.

project_path: .                  # target project root
features_file: ./FEATURES.md    # feature spec (kept in project root for visibility)
provider: claude                 # claude | copilot
fallback_provider: ""            # optional secondary provider; empty = none
model: ""                        # provider-specific model; empty = provider default
task_timeout: 30m                # per-feature subprocess timeout
auto_commit: false               # require explicit opt-in before committing
max_retries: 1                   # retries per feature on failure

database:
  driver: sqlite                 # sqlite | postgres | turso
  dsn: ".pave/state.db"         # path relative to current directory (sqlite) or connection URL

limiter:
  window: 5h                     # rolling usage window length
  backoff_initial: 1m            # first backoff interval
  backoff_max: 5h                # cap on backoff

# ui:
#   path: ".pave/ui"             # directory where the UI assets are stored
#   port: 4000                   # port for the local UI server
`

// DefaultFeaturesMD is the starter FEATURES.md content.
const DefaultFeaturesMD = `# Features

List the features you want pave to implement. Each item is a task-list entry:

- ` + "`- [ ]`" + ` = pending, ` + "`- [x]`" + ` = already implemented.
- Add an optional description after an em dash: ` + "`Title — description`" + `.
- Add optional metadata in parentheses: ` + "`(priority: 1, depends: other-id)`" + `.

## Backlog

- [ ] Example feature — replace this with your first real feature (priority: 1)
`

// Result reports which files an Init call created versus skipped.
type Result struct {
	PaveDir         string
	ConfigPath      string
	ConfigCreated   bool
	FeaturesPath    string
	FeaturesCreated bool
}

// Init scaffolds .pave/pave.yaml and FEATURES.md under dir. The config and
// database live inside .pave/ so the whole directory can be gitignored. The
// FEATURES.md is placed in dir itself so it remains visible in the project.
// Neither file is overwritten if it already exists.
func Init(dir string) (Result, error) {
	paveDir := filepath.Join(dir, ".pave")
	if err := os.MkdirAll(paveDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("creating .pave directory: %w", err)
	}

	res := Result{
		PaveDir:      paveDir,
		ConfigPath:   filepath.Join(paveDir, "pave.yaml"),
		FeaturesPath: filepath.Join(dir, "FEATURES.md"),
	}

	created, err := writeIfAbsent(res.ConfigPath, DefaultConfigYAML)
	if err != nil {
		return res, err
	}
	res.ConfigCreated = created

	created, err = writeIfAbsent(res.FeaturesPath, DefaultFeaturesMD)
	if err != nil {
		return res, err
	}
	res.FeaturesCreated = created

	return res, nil
}

// writeIfAbsent writes content to path only if path does not already exist.
// It returns whether the file was created.
func writeIfAbsent(path, content string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil // already exists; leave it untouched
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("checking %q: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, fmt.Errorf("writing %q: %w", path, err)
	}
	return true, nil
}
