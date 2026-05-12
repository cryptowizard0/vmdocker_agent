package claude

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cryptowizard0/vmdocker_agent/harness"
	"github.com/cryptowizard0/vmdocker_agent/runtime/backend"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

func TestBackendApplyUsesExistingClaudeRuntime(t *testing.T) {
	workspace := t.TempDir()
	cliPath := filepath.Join(t.TempDir(), "claude")
	script := "#!/bin/sh\nprintf '{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"result\":\"ok from backend\",\"session_id\":\"sess-backend\"}'\n"
	if err := os.WriteFile(cliPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude failed: %v", err)
	}

	t.Setenv("CLAUDE_CODE_BIN", cliPath)
	t.Setenv("VMDOCKER_AGENT_WORKSPACE", workspace)

	b, err := New(backend.Config{
		Harness: harness.Context{
			ProfileName: "claude",
			Backend:     "claude",
			Workspace:   workspace,
		},
		SpawnParams: map[string]string{"model": "claude-test"},
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	resp, err := b.Apply(backend.Request{
		From:   "target-1",
		Meta:   vmmSchema.Meta{Action: "Chat"},
		Params: map[string]string{"Command": "hello"},
	})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if resp.Result.Data != "ok from backend" {
		t.Fatalf("data = %q, want ok from backend", resp.Result.Data)
	}
}
