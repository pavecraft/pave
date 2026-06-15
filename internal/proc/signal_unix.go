//go:build unix

package proc

import (
	"errors"
	"os/exec"
	"syscall"
)

// setPgid makes the child the leader of a new process group so signals can be
// delivered to the whole group (the CLI plus any children it spawns).
func setPgid(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// signalGroup sends sig to the child's process group.
func (p *Process) signalGroup(sig syscall.Signal) error {
	if p.cmd.Process == nil {
		return errors.New("process not started")
	}
	// With Setpgid, the child's PGID equals its PID; negate to target the group.
	return syscall.Kill(-p.cmd.Process.Pid, sig)
}

func (p *Process) pause() error     { return p.signalGroup(syscall.SIGSTOP) }
func (p *Process) resume() error    { return p.signalGroup(syscall.SIGCONT) }
func (p *Process) terminate() error { return p.signalGroup(syscall.SIGTERM) }
func (p *Process) kill() error      { return p.signalGroup(syscall.SIGKILL) }
