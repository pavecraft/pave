//go:build !unix

package proc

import (
	"errors"
	"os/exec"
)

// errNoJobControl is returned for pause/resume on platforms (e.g. Windows)
// without POSIX job-control signals.
var errNoJobControl = errors.New("pause/resume not supported on this platform")

// setPgid is a no-op on platforms without process groups.
func setPgid(cmd *exec.Cmd) {}

func (p *Process) pause() error  { return errNoJobControl }
func (p *Process) resume() error { return nil } // nothing to resume; safe no-op

func (p *Process) terminate() error {
	if p.cmd.Process == nil {
		return errors.New("process not started")
	}
	return p.cmd.Process.Kill()
}

func (p *Process) kill() error {
	if p.cmd.Process == nil {
		return errors.New("process not started")
	}
	return p.cmd.Process.Kill()
}
