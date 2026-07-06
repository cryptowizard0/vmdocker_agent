// supervisor/supervisor_test.go
package supervisor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartCapturesOutput(t *testing.T) {
	dir := t.TempDir()
	hook := filepath.Join(dir, "start.sh")
	logPath := filepath.Join(dir, "start.log")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho hello-stdout\necho hello-stderr 1>&2\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	s := New(hook, logPath)
	if err := s.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	var data []byte
	for time.Now().Before(deadline) {
		data, _ = os.ReadFile(logPath)
		if strings.Contains(string(data), "hello-stdout") && strings.Contains(string(data), "hello-stderr") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("log did not capture output, got %q", string(data))
}

func TestStartMissingHookIsNoOp(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "nope.sh"), filepath.Join(t.TempDir(), "x.log"))
	if err := s.Start(); err != nil {
		t.Fatalf("missing hook should be a no-op, got %v", err)
	}
	if s.PGID() != 0 {
		t.Fatalf("no process should have been started, PGID=%d", s.PGID())
	}
}
