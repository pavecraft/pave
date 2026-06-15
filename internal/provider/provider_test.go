package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pavecraft/pave/internal/project"
)

// compile-time check that Claude satisfies the interface.
var _ Provider = (*Claude)(nil)

func TestBuildPrompt(t *testing.T) {
	t.Parallel()
	task := Task{
		Feature: project.Feature{
			ID:          "f01-config",
			Title:       "Config loader",
			Description: "Load and validate pave.yaml",
			DependsOn:   []string{"f00-init"},
		},
		Context: "Use gopkg.in/yaml.v3.",
	}
	got := BuildPrompt(task)

	for _, want := range []string{
		"Config loader",
		"f01-config",
		"Load and validate pave.yaml",
		"f00-init",
		"Use gopkg.in/yaml.v3.",
		"Implement ONLY this feature",
		"Do not commit",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q\n---\n%s", want, got)
		}
	}
}

func TestBuildPromptMinimal(t *testing.T) {
	t.Parallel()
	got := BuildPrompt(Task{Feature: project.Feature{Title: "Bare"}})
	if !strings.Contains(got, "Bare") {
		t.Errorf("prompt missing title: %s", got)
	}
	// No description / deps / context sections when fields are empty.
	if strings.Contains(got, "Description:") {
		t.Error("unexpected Description section for empty description")
	}
	if strings.Contains(got, "Additional context") {
		t.Error("unexpected context section for empty context")
	}
}

func TestClaudeName(t *testing.T) {
	t.Parallel()
	if NewClaude().Name() != "claude" {
		t.Error("Name() != claude")
	}
}

func TestClaudeBuildArgs(t *testing.T) {
	t.Parallel()
	c := NewClaude()
	args := c.buildArgs(Task{Feature: project.Feature{Title: "X"}, Model: "sonnet"})
	if len(args) < 2 || args[0] != "-p" {
		t.Fatalf("args = %v, want leading -p <prompt>", args)
	}
	if args[len(args)-2] != "--model" || args[len(args)-1] != "sonnet" {
		t.Errorf("model flag missing: %v", args)
	}

	noModel := c.buildArgs(Task{Feature: project.Feature{Title: "X"}})
	for _, a := range noModel {
		if a == "--model" {
			t.Error("did not expect --model when Model is empty")
		}
	}
}

func TestClaudeAvailable(t *testing.T) {
	t.Parallel()
	c := &Claude{Bin: "definitely-not-a-real-binary-xyz"}
	if err := c.Available(context.Background()); err == nil {
		t.Error("expected error for missing binary")
	}
}

// fakeClaude writes a script named "claude" into a temp dir, returns its dir.
// The script echoes a marker and exits with the given code.
func fakeClaude(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\n" + body + "\n"
	path := filepath.Join(dir, "claude")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writing fake claude: %v", err)
	}
	return dir
}

func TestClaudeRunSuccess(t *testing.T) {
	t.Parallel()
	dir := fakeClaude(t, `echo "implemented"; exit 0`)
	c := &Claude{Bin: filepath.Join(dir, "claude")}

	res, err := c.Run(context.Background(), Task{
		Feature:     project.Feature{Title: "Thing"},
		ProjectPath: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	if !res.Success || res.RawExit != 0 {
		t.Errorf("result = %+v, want success", res)
	}
	if !strings.Contains(res.Output, "implemented") {
		t.Errorf("output = %q", res.Output)
	}
}

func TestClaudeRunFailure(t *testing.T) {
	t.Parallel()
	dir := fakeClaude(t, `echo "rate limit reached" 1>&2; exit 1`)
	c := &Claude{Bin: filepath.Join(dir, "claude")}

	res, err := c.Run(context.Background(), Task{
		Feature:     project.Feature{Title: "Thing"},
		ProjectPath: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	if res.Success || res.RawExit != 1 {
		t.Errorf("result = %+v, want failure exit 1", res)
	}
	if !strings.Contains(res.Stderr, "rate limit") {
		t.Errorf("stderr = %q", res.Stderr)
	}
}

func TestClaudeRunsInProjectDir(t *testing.T) {
	t.Parallel()
	dir := fakeClaude(t, `pwd`)
	proj := t.TempDir()
	c := &Claude{Bin: filepath.Join(dir, "claude")}

	res, err := c.Run(context.Background(), Task{
		Feature:     project.Feature{Title: "Thing"},
		ProjectPath: proj,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want, _ := filepath.EvalSymlinks(proj)
	got, _ := filepath.EvalSymlinks(strings.TrimSpace(res.Output))
	if got != want {
		t.Errorf("ran in %q, want %q", got, want)
	}
}
