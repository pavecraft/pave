package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pavecraft/pave/internal/project"
)

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestScanFindsReferencedIDs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "auth.go", "// implements f01-login\npackage main\n")
	writeFile(t, dir, "sub/notes.md", "TODO: start f02-logout\n")

	features := []project.Feature{
		{ID: "f01-login"},
		{ID: "f02-logout"},
		{ID: "f03-unused"},
	}
	found, err := Scan(dir, features)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !found["f01-login"] {
		t.Error("expected f01-login found")
	}
	if !found["f02-logout"] {
		t.Error("expected f02-logout found")
	}
	if found["f03-unused"] {
		t.Error("did not expect f03-unused found")
	}
}

func TestScanSkipsIgnoredDirs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "node_modules/pkg/index.js", "f01-login everywhere\n")
	writeFile(t, dir, ".git/config", "f01-login\n")

	found, err := Scan(dir, []project.Feature{{ID: "f01-login"}})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if found["f01-login"] {
		t.Error("should not match inside skipped directories")
	}
}

func TestScanEmptyFeatures(t *testing.T) {
	t.Parallel()
	found, err := Scan(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(found) != 0 {
		t.Errorf("expected empty result, got %v", found)
	}
}

func TestRefineUpgradesPendingOnly(t *testing.T) {
	t.Parallel()
	features := []project.Feature{
		{ID: "a", Status: project.StatusPending},
		{ID: "b", Status: project.StatusPending},
		{ID: "c", Status: project.StatusFailed},
	}
	found := map[string]bool{"a": true, "b": false, "c": true}

	out := Refine(features, found)
	if out[0].Status != project.StatusImplemented {
		t.Errorf("a = %q, want implemented", out[0].Status)
	}
	if out[1].Status != project.StatusPending {
		t.Errorf("b = %q, want pending", out[1].Status)
	}
	// c was failed; scanner must not override a non-pending status.
	if out[2].Status != project.StatusFailed {
		t.Errorf("c = %q, want failed (unchanged)", out[2].Status)
	}
	// Input not mutated.
	if features[0].Status != project.StatusPending {
		t.Error("Refine mutated its input")
	}
}
