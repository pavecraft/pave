package scaffold

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCreatesFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	res, err := Init(dir)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if !res.ConfigCreated || !res.FeaturesCreated {
		t.Fatalf("expected both files created, got %+v", res)
	}

	// Config must be inside .pave/
	cfg, err := os.ReadFile(filepath.Join(dir, ".pave", "pave.yaml"))
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	if string(cfg) != DefaultConfigYAML {
		t.Error("config content does not match default")
	}
	// FEATURES.md stays in the project root
	if _, err := os.Stat(filepath.Join(dir, "FEATURES.md")); err != nil {
		t.Errorf("FEATURES.md not created: %v", err)
	}
}

func TestInitIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Pre-existing config inside .pave/ must not be overwritten.
	custom := "provider: copilot\n"
	paveDir := filepath.Join(dir, ".pave")
	if err := os.MkdirAll(paveDir, 0o755); err != nil {
		t.Fatalf("creating .pave: %v", err)
	}
	cfgPath := filepath.Join(paveDir, "pave.yaml")
	if err := os.WriteFile(cfgPath, []byte(custom), 0o644); err != nil {
		t.Fatalf("seeding config: %v", err)
	}

	res, err := Init(dir)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if res.ConfigCreated {
		t.Error("expected existing config to be left untouched")
	}
	if !res.FeaturesCreated {
		t.Error("expected FEATURES.md to be created")
	}

	got, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("re-reading config: %v", err)
	}
	if string(got) != custom {
		t.Errorf("config was overwritten: got %q", got)
	}
}

func TestInitSecondRunCreatesNothing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	if _, err := Init(dir); err != nil {
		t.Fatalf("first Init() error = %v", err)
	}
	res, err := Init(dir)
	if err != nil {
		t.Fatalf("second Init() error = %v", err)
	}
	if res.ConfigCreated || res.FeaturesCreated {
		t.Errorf("second run should create nothing, got %+v", res)
	}
}
