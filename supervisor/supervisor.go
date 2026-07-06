// supervisor/supervisor.go
package supervisor

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

// Supervisor launches the untrusted start.sh in its own process group and
// captures its output. The adapter (PID 1) uses PGID to forward signals to the
// engine even after start.sh exits.
type Supervisor struct {
	hookPath string
	logPath  string

	mu   sync.Mutex
	cmd  *exec.Cmd
	pgid int
	// reap is the zombie-reaping function; injectable for tests. Defaults to
	// reapZombies. See Task 3.
	reap func()
}

func New(hookPath, logPath string) *Supervisor {
	return &Supervisor{hookPath: hookPath, logPath: logPath, reap: reapZombies}
}

// Start launches `sh <hookPath>` with stdout+stderr redirected to logPath in a
// new process group. A missing hook is a no-op.
func (s *Supervisor) Start() error {
	if _, err := os.Stat(s.hookPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat start hook: %w", err)
	}

	logFile, err := os.OpenFile(s.logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open start log: %w", err)
	}

	cmd := exec.Command("sh", s.hookPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start start.sh: %w", err)
	}
	logFile.Close() // child holds its own dup of the fd; parent's copy not needed

	s.mu.Lock()
	s.cmd = cmd
	s.pgid = cmd.Process.Pid // equals the new pgid because Setpgid is set
	s.mu.Unlock()
	return nil
}

func (s *Supervisor) PGID() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pgid
}

// reapZombies drains all reapable children. As PID 1 the adapter inherits
// orphaned engine processes; this prevents zombie accumulation.
func reapZombies() {
	for {
		var ws syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &ws, syscall.WNOHANG, nil)
		if pid <= 0 || err != nil {
			return
		}
	}
}

// ReapLoop reaps children whenever a SIGCHLD arrives on sigchld.
func (s *Supervisor) ReapLoop(sigchld <-chan os.Signal) {
	for range sigchld {
		s.reap()
	}
}

// Forward sends sig to the start.sh process group (negative pgid). No-op if no
// process was started.
func (s *Supervisor) Forward(sig syscall.Signal) error {
	pgid := s.PGID()
	if pgid == 0 {
		return nil
	}
	if err := syscall.Kill(-pgid, sig); err != nil {
		return fmt.Errorf("signal process group %d: %w", pgid, err)
	}
	return nil
}
