package runtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	schema "github.com/cryptowizard0/vmdocker_agent/runtime/openclaw/schema"
	"github.com/cryptowizard0/vmdocker_agent/runtime/telegramcustomer"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

func TestNewRuntimeOpenclaw(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok","data":"pong"}`))
			return
		}
		if r.URL.Path == "/tools/invoke" {
			var req schema.ToolInvokeRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Tool == "sessions_create" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"ok","data":{"sessionId":"runtime-sess-1"}}`))
				return
			}
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"status":"error"}`))
	}))
	defer gateway.Close()

	t.Setenv("RUNTIME_TYPE", RuntimeTypeOpenclaw)
	t.Setenv("OPENCLAW_GATEWAY_URL", gateway.URL)
	t.Setenv("OPENCLAW_TIMEOUT_MS", "1000")
	setupRuntimeProfileEnv(t, "")

	rt, err := New(vmmSchema.Env{}, "", "", nil, nil)
	if err != nil {
		t.Fatalf("new runtime failed: %v", err)
	}
	if rt == nil || rt.backend == nil {
		t.Fatalf("runtime backend is nil")
	}
}

func TestNewRestoredRuntimeOpenclaw(t *testing.T) {
	createCalled := false
	sendSessionKey := ""

	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok","data":"pong"}`))
			return
		}
		if r.URL.Path == "/tools/invoke" {
			var req schema.ToolInvokeRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			switch req.Tool {
			case "sessions_create":
				createCalled = true
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"ok","data":{"sessionId":"unexpected"}}`))
				return
			case "sessions_send":
				sendSessionKey = req.SessionKey
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"ok","data":"restored"}`))
				return
			}
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"status":"error"}`))
	}))
	defer gateway.Close()

	t.Setenv("RUNTIME_TYPE", RuntimeTypeOpenclaw)
	t.Setenv("OPENCLAW_GATEWAY_URL", gateway.URL)
	t.Setenv("OPENCLAW_TIMEOUT_MS", "1000")
	setupRuntimeProfileEnv(t, "")

	rt, err := NewRestored(vmmSchema.Env{}, "", "", nil, `{"format":"openclaw.runtime.v1","sessionId":"runtime-restored-1"}`)
	if err != nil {
		t.Fatalf("new restored runtime failed: %v", err)
	}
	if rt == nil || rt.backend == nil {
		t.Fatalf("runtime backend is nil")
	}

	if _, err := rt.Apply("target-1", vmmSchema.Meta{Action: "Execute"}, map[string]string{"Command": "hi"}); err != nil {
		t.Fatalf("apply after runtime restore failed: %v", err)
	}
	if createCalled {
		t.Fatalf("did not expect sessions_create in restore path")
	}
	if sendSessionKey != "runtime-restored-1" {
		t.Fatalf("expected restored session key runtime-restored-1, got %q", sendSessionKey)
	}
}

func TestNewRuntimeClaude(t *testing.T) {
	setupRuntimeProfileEnv(t, "")
	cliPath := filepath.Join(t.TempDir(), "claude")
	script := "#!/bin/sh\nprintf '{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"result\":\"ok\",\"session_id\":\"claude-session-1\"}'\n"
	if err := os.WriteFile(cliPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude failed: %v", err)
	}

	t.Setenv("RUNTIME_TYPE", RuntimeTypeClaude)
	t.Setenv("CLAUDE_CODE_BIN", cliPath)
	t.Setenv("ANTHROPIC_MODEL", "test-model")

	rt, err := New(vmmSchema.Env{}, "", "", nil, map[string]string{"model": "qwen3.5-plus"})
	if err != nil {
		t.Fatalf("new runtime failed: %v", err)
	}
	if rt == nil || rt.backend == nil {
		t.Fatalf("runtime backend is nil")
	}
}

func TestNewRestoredRuntimeClaude(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "claude.log")
	setupRuntimeProfileEnv(t, "")
	cliPath := filepath.Join(t.TempDir(), "claude")
	script := "#!/bin/sh\nprintf '%s\n' \"$*\" >>" + shellQuoteForTest(logPath) + "\nprintf '{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"result\":\"restored\",\"session_id\":\"runtime-restored-1\"}'\n"
	if err := os.WriteFile(cliPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude failed: %v", err)
	}

	t.Setenv("RUNTIME_TYPE", RuntimeTypeClaude)
	t.Setenv("CLAUDE_CODE_BIN", cliPath)

	rt, err := NewRestored(vmmSchema.Env{}, "", "", nil, `{"format":"claudecode.runtime.v1","sessionId":"runtime-restored-1"}`)
	if err != nil {
		t.Fatalf("new restored runtime failed: %v", err)
	}
	if rt == nil || rt.backend == nil {
		t.Fatalf("runtime backend is nil")
	}

	if _, err := rt.Apply("target-1", vmmSchema.Meta{Action: "Execute"}, map[string]string{"Command": "hi"}); err != nil {
		t.Fatalf("apply after runtime restore failed: %v", err)
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake claude log failed: %v", err)
	}
	if !strings.Contains(string(raw), "--resume runtime-restored-1") {
		t.Fatalf("expected restored session id in log, got %s", string(raw))
	}
}

func TestNewRuntimeTelegramCustomer(t *testing.T) {
	runtimeRoot := setupRuntimeProfileEnv(t, "")
	workspace := filepath.Join(runtimeRoot, "workspace")
	homeDir := filepath.Join(runtimeRoot, ".home")
	cliPath := filepath.Join(t.TempDir(), "claude")
	script := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(cliPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude failed: %v", err)
	}

	originalRunByHymx := telegramcustomer.RunByHymx
	telegramcustomer.RunByHymx = func(workspaceDir, runtimeHomeDir, botToken string) error {
		if workspaceDir != workspace {
			t.Fatalf("expected workspace %q, got %q", workspace, workspaceDir)
		}
		if runtimeHomeDir != homeDir {
			t.Fatalf("expected runtime home %q, got %q", homeDir, runtimeHomeDir)
		}
		if botToken != "bot-token" {
			t.Fatalf("expected bot token to be forwarded")
		}
		return nil
	}
	defer func() {
		telegramcustomer.RunByHymx = originalRunByHymx
	}()

	t.Setenv("RUNTIME_TYPE", RuntimeTypeTelegramCustomer)
	t.Setenv("CLAUDE_CODE_BIN", cliPath)
	t.Setenv("BOT_TOKEN", "bot-token")

	rt, err := New(vmmSchema.Env{}, "", "", nil, map[string]string{"model": "qwen3.5-plus"})
	if err != nil {
		t.Fatalf("new telegramcustomer runtime failed: %v", err)
	}
	if rt == nil || rt.backend == nil {
		t.Fatalf("runtime backend is nil")
	}

	state, err := rt.Checkpoint()
	if err != nil {
		t.Fatalf("checkpoint failed: %v", err)
	}
	if !strings.Contains(state, "telegramcustomer.runtime.v1") {
		t.Fatalf("expected telegramcustomer checkpoint format, got %s", state)
	}
}

func TestNewRuntimeClaudeViaAgentProfile(t *testing.T) {
	runtimeRoot := setupRuntimeProfileEnv(t, "claude")
	logPath := filepath.Join(t.TempDir(), "claude-env.log")
	cliPath := filepath.Join(t.TempDir(), "claude")
	script := "#!/bin/sh\nprintf 'role=%s\\ncontext=%s\\n' \"$VMDOCKER_AGENT_ROLE_PATH\" \"$VMDOCKER_AGENT_CONTEXT_DIR\" >" + shellQuoteForTest(logPath) + "\nprintf '{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"result\":\"profile ok\",\"session_id\":\"sess-profile\"}'\n"
	if err := os.WriteFile(cliPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude failed: %v", err)
	}
	t.Setenv("CLAUDE_CODE_BIN", cliPath)

	rt, err := New(vmmSchema.Env{}, "", "", nil, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	result, err := rt.Apply("target-1", vmmSchema.Meta{Action: "Chat"}, map[string]string{"Command": "hello"})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if !strings.Contains(result, "profile ok") {
		t.Fatalf("expected profile result, got %s", result)
	}
	rawEnv, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake claude env log failed: %v", err)
	}
	envLog := string(rawEnv)
	agentRoot := filepath.Join(runtimeRoot, ".vmdocker-agent")
	if !strings.Contains(envLog, "role="+filepath.Join(agentRoot, "roles", "claude.md")) {
		t.Fatalf("expected role path under .vmdocker-agent, got %s", envLog)
	}
	if !strings.Contains(envLog, "context="+filepath.Join(agentRoot, "context")) {
		t.Fatalf("expected context path under .vmdocker-agent, got %s", envLog)
	}
	state, err := rt.Checkpoint()
	if err != nil {
		t.Fatalf("Checkpoint failed: %v", err)
	}
	if !strings.Contains(state, checkpointEnvelopeFormatV1) {
		t.Fatalf("expected checkpoint envelope, got %s", state)
	}
}

func TestNewRuntimeFallsBackToRuntimeTypeClaude(t *testing.T) {
	setupRuntimeProfileEnv(t, "")
	cliPath := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(cliPath, []byte("#!/bin/sh\nprintf '{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"result\":\"ok\",\"session_id\":\"sess\"}'\n"), 0o755); err != nil {
		t.Fatalf("write fake claude failed: %v", err)
	}

	t.Setenv("RUNTIME_TYPE", RuntimeTypeClaude)
	t.Setenv("CLAUDE_CODE_BIN", cliPath)

	rt, err := New(vmmSchema.Env{}, "", "", nil, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if rt.profileName != "claude" {
		t.Fatalf("profileName = %q, want claude", rt.profileName)
	}
}

func TestApplyHarnessEnvDoesNotMutateSelectorEnv(t *testing.T) {
	for _, key := range harnessEnvKeys {
		t.Setenv(key, "")
	}
	t.Setenv("VMDOCKER_AGENT_PROFILE", "")
	t.Setenv("VMDOCKER_AGENT_PROFILE_DIR", "/caller/profile-dir")
	t.Setenv("RUNTIME_TYPE", RuntimeTypeClaude)

	env := map[string]string{
		"VMDOCKER_AGENT_PROFILE":     "test",
		"VMDOCKER_AGENT_PROFILE_DIR": "/runtime/.vmdocker-agent/profiles/test",
		"RUNTIME_TYPE":               RuntimeTypeTest,
		"VMDOCKER_AGENT_ASSET_ROOT":  "/runtime/.vmdocker-agent",
		"VMDOCKER_AGENT_ROLE_PATH":   "/runtime/.vmdocker-agent/roles/test.md",
		"VMDOCKER_AGENT_CONTEXT_DIR": "/runtime/.vmdocker-agent/context",
		"VMDOCKER_AGENT_MEMORY_DIR":  "/runtime/.vmdocker-agent/memory",
		"VMDOCKER_AGENT_SKILLS_DIR":  "/runtime/.vmdocker-agent/skills",
	}

	if err := applyHarnessEnv(env); err != nil {
		t.Fatalf("applyHarnessEnv failed: %v", err)
	}
	if got := os.Getenv("VMDOCKER_AGENT_PROFILE"); got != "" {
		t.Fatalf("VMDOCKER_AGENT_PROFILE = %q, want unchanged empty value", got)
	}
	if got := os.Getenv("VMDOCKER_AGENT_PROFILE_DIR"); got != "/caller/profile-dir" {
		t.Fatalf("VMDOCKER_AGENT_PROFILE_DIR = %q, want caller value", got)
	}
	if got := os.Getenv("RUNTIME_TYPE"); got != RuntimeTypeClaude {
		t.Fatalf("RUNTIME_TYPE = %q, want caller value", got)
	}
	if got := os.Getenv("VMDOCKER_AGENT_ROLE_PATH"); got != "/runtime/.vmdocker-agent/roles/test.md" {
		t.Fatalf("VMDOCKER_AGENT_ROLE_PATH = %q", got)
	}
	if got := os.Getenv("VMDOCKER_AGENT_CONTEXT_DIR"); got != "/runtime/.vmdocker-agent/context" {
		t.Fatalf("VMDOCKER_AGENT_CONTEXT_DIR = %q", got)
	}
}

func setupRuntimeProfileEnv(t *testing.T, profileName string) string {
	t.Helper()

	runtimeRoot := t.TempDir()
	assetRoot := filepath.Join(runtimeRoot, ".vmdocker-agent")
	profileRoot := filepath.Join(assetRoot, "profiles")
	copyDirForRuntimeTest(t, filepath.Join("..", "harness", "profiles"), profileRoot)
	copyDirForRuntimeTest(t, filepath.Join("..", "harness", "roles"), filepath.Join(assetRoot, "roles"))
	copyDirForRuntimeTest(t, filepath.Join("..", "harness", "skills"), filepath.Join(assetRoot, "skills"))

	if profileName != "" {
		t.Setenv("VMDOCKER_AGENT_PROFILE", profileName)
	}
	t.Setenv("VMDOCKER_AGENT_PROFILE_DIR", profileRoot)
	t.Setenv("VMDOCKER_RUNTIME_WORKSPACE", runtimeRoot)
	t.Setenv("VMDOCKER_AGENT_WORKSPACE", filepath.Join(runtimeRoot, "workspace"))
	t.Setenv("VMDOCKER_RUNTIME_HOME", filepath.Join(runtimeRoot, ".home"))
	for _, key := range harnessEnvKeys {
		t.Setenv(key, "")
	}

	return runtimeRoot
}

func copyDirForRuntimeTest(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read dir %s failed: %v", src, err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir %s failed: %v", dst, err)
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			copyDirForRuntimeTest(t, srcPath, dstPath)
			continue
		}
		data, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatalf("read %s failed: %v", srcPath, err)
		}
		if err := os.WriteFile(dstPath, data, 0o644); err != nil {
			t.Fatalf("write %s failed: %v", dstPath, err)
		}
	}
}

func shellQuoteForTest(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
