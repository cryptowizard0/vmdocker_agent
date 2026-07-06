// startup/templates_test.go
package startup_test

import (
	"os"
	"os/exec"
	"testing"
)

func TestOpenclawTemplateIsValidShell(t *testing.T) {
	if _, err := os.Stat("openclaw.sh"); err != nil {
		t.Fatalf("openclaw.sh missing: %v", err)
	}
	if out, err := exec.Command("sh", "-n", "openclaw.sh").CombinedOutput(); err != nil {
		t.Fatalf("openclaw.sh not valid sh: %v\n%s", err, out)
	}
}

func TestClaudeTemplateIsValidShell(t *testing.T) {
	if _, err := os.Stat("claude.sh"); err != nil {
		t.Fatalf("claude.sh missing: %v", err)
	}
	if out, err := exec.Command("sh", "-n", "claude.sh").CombinedOutput(); err != nil {
		t.Fatalf("claude.sh not valid sh: %v\n%s", err, out)
	}
}
