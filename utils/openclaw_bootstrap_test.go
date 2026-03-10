package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveOpenclawStateDir(t *testing.T) {
	fakeHome := func() (string, error) { return "/user-home", nil }

	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "state dir override wins",
			env: map[string]string{
				"OPENCLAW_STATE_DIR": "/sandbox/state",
				"OPENCLAW_HOME":      "/ignored-home",
				"HOME":               "/ignored",
			},
			want: "/sandbox/state",
		},
		{
			name: "openclaw home derives state dir",
			env: map[string]string{
				"OPENCLAW_HOME": "/sandbox/home",
			},
			want: "/sandbox/home/.openclaw",
		},
		{
			name: "home derives state dir",
			env: map[string]string{
				"HOME": "/sandbox/user",
			},
			want: "/sandbox/user/.openclaw",
		},
		{
			name: "slash home falls back to user home dir",
			env: map[string]string{
				"HOME": "/",
			},
			want: "/user-home/.openclaw",
		},
		{
			name: "missing home falls back to tmp",
			env:  map[string]string{},
			want: "/user-home/.openclaw",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveOpenclawStateDir(func(key string) string {
				return tt.env[key]
			}, fakeHome)
			if got != tt.want {
				t.Fatalf("ResolveOpenclawStateDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveOpenclawStateDirFallbackToTmp(t *testing.T) {
	got := ResolveOpenclawStateDir(func(string) string { return "" }, func() (string, error) {
		return "/", nil
	})
	if got != "/tmp/.openclaw" {
		t.Fatalf("ResolveOpenclawStateDir() = %q, want /tmp/.openclaw", got)
	}
}

func TestPrepareOpenclawRuntimeMaterializesConfigOnce(t *testing.T) {
	tempDir := t.TempDir()
	templatePath := filepath.Join(tempDir, "template.json")
	if err := os.WriteFile(templatePath, []byte("{\n  \"gateway\": {\n    \"mode\": \"local\"\n  }\n}\n"), 0o644); err != nil {
		t.Fatalf("write template failed: %v", err)
	}

	configPath := filepath.Join(tempDir, "state", "openclaw.json")
	env := map[string]string{
		"OPENCLAW_STATE_DIR":            filepath.Join(tempDir, "state"),
		"OPENCLAW_CONFIG_TEMPLATE_PATH": templatePath,
		"OPENCLAW_CONFIG_PATH":          configPath,
		"OPENCLAW_GATEWAY_MODE":         "proxy",
	}

	paths, err := PrepareOpenclawRuntime(func(key string) string { return env[key] }, nil)
	if err != nil {
		t.Fatalf("PrepareOpenclawRuntime() failed: %v", err)
	}

	if paths.StateDir != env["OPENCLAW_STATE_DIR"] {
		t.Fatalf("state dir = %q, want %q", paths.StateDir, env["OPENCLAW_STATE_DIR"])
	}
	if paths.ConfigPath != configPath {
		t.Fatalf("config path = %q, want %q", paths.ConfigPath, configPath)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	if string(raw) != "{\n  \"gateway\": {\n    \"mode\": \"proxy\"\n  }\n}\n" {
		t.Fatalf("unexpected materialized config: %s", string(raw))
	}

	if err := os.WriteFile(configPath, []byte("{\"gateway\":{\"mode\":\"custom\"}}\n"), 0o600); err != nil {
		t.Fatalf("rewrite config failed: %v", err)
	}

	if _, err := PrepareOpenclawRuntime(func(key string) string { return env[key] }, nil); err != nil {
		t.Fatalf("second PrepareOpenclawRuntime() failed: %v", err)
	}

	raw, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after second prepare failed: %v", err)
	}
	if string(raw) != "{\"gateway\":{\"mode\":\"custom\"}}\n" {
		t.Fatalf("config was overwritten on second prepare: %s", string(raw))
	}

	sessionsPath := filepath.Join(env["OPENCLAW_STATE_DIR"], "agents", "main", "sessions", "sessions.json")
	if _, err := os.Stat(sessionsPath); err != nil {
		t.Fatalf("sessions store missing: %v", err)
	}
}
