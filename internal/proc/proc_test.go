package proc

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestStartCapturesOutputAndExitCode(t *testing.T) {
	t.Parallel()
	p, err := Start(context.Background(), "sh", []string{"-c", "echo out; echo err 1>&2; exit 3"}, "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	code, werr := p.Wait()
	if werr != nil {
		t.Fatalf("Wait error = %v", werr)
	}
	if code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
	if got := p.StdoutString(); got != "out\n" {
		t.Errorf("stdout = %q, want %q", got, "out\n")
	}
	if got := p.StderrString(); got != "err\n" {
		t.Errorf("stderr = %q, want %q", got, "err\n")
	}
}

func TestStartUsesWorkingDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p, err := Start(context.Background(), "sh", []string{"-c", "pwd"}, dir)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := p.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	// macOS resolves TempDir under /private; compare by suffix via EvalSymlinks.
	want, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(trimNewline(p.StdoutString()))
	if got != want {
		t.Errorf("pwd = %q, want %q", got, want)
	}
}

func TestStopTerminatesLongRunning(t *testing.T) {
	t.Parallel()
	p, err := Start(context.Background(), "sleep", []string{"30"}, "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	start := time.Now()
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	code, _ := p.Wait()
	if time.Since(start) > stopGrace {
		t.Errorf("Stop took too long: %v", time.Since(start))
	}
	if code == 0 {
		t.Errorf("expected nonzero exit code after Stop, got 0")
	}
}

func TestContextCancelStops(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	p, err := Start(ctx, "sleep", []string{"30"}, "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	cancel()

	select {
	case <-p.Done():
	case <-time.After(stopGrace + 2*time.Second):
		t.Fatal("process did not stop after context cancel")
	}
}

func TestStopIsIdempotentAfterExit(t *testing.T) {
	t.Parallel()
	p, err := Start(context.Background(), "sh", []string{"-c", "exit 0"}, "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := p.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if err := p.Stop(); err != nil {
		t.Errorf("Stop after exit = %v, want nil", err)
	}
}

func TestPauseResume(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("job-control signals not supported on Windows")
	}
	t.Parallel()
	dir := t.TempDir()
	marker := filepath.Join(dir, "done")
	// The script sleeps briefly then creates the marker file.
	p, err := Start(context.Background(), "sh", []string{"-c", "sleep 0.5; touch '" + marker + "'"}, "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Pause immediately, before the 0.5s sleep elapses.
	if err := p.Pause(); err != nil {
		t.Fatalf("Pause: %v", err)
	}

	// Wait well past the natural completion time; while paused the marker must
	// not appear.
	time.Sleep(1200 * time.Millisecond)
	if fileExists(marker) {
		t.Fatal("marker created while process was paused")
	}

	if err := p.Resume(); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if _, err := p.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if !fileExists(marker) {
		t.Error("marker not created after resume")
	}
}

func TestStartIOStreamsAndPassesEnv(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	p, err := StartIO(context.Background(), "sh", []string{"-c", "echo $PAVE_TEST_VAR"}, "", IOOptions{
		Stdout: &out,
		Env:    []string{"PAVE_TEST_VAR=hello"},
	})
	if err != nil {
		t.Fatalf("StartIO: %v", err)
	}
	if _, err := p.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if got := trimNewline(out.String()); got != "hello" {
		t.Errorf("streamed output = %q, want %q", got, "hello")
	}
}

func TestStartIOStopsOnContextCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	p, err := StartIO(ctx, "sleep", []string{"30"}, "", IOOptions{})
	if err != nil {
		t.Fatalf("StartIO: %v", err)
	}
	cancel()
	select {
	case <-p.Done():
	case <-time.After(stopGrace + 2*time.Second):
		t.Fatal("StartIO process did not stop after cancel")
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
