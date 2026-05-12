package profile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSelectorPrefersAgentProfile(t *testing.T) {
	t.Setenv("VMDOCKER_AGENT_PROFILE", "custom")
	t.Setenv("RUNTIME_TYPE", "claude")

	got := ResolveSelector(os.Getenv)
	if got != "custom" {
		t.Fatalf("selector = %q, want custom", got)
	}
}

func TestResolveSelectorMapsRuntimeType(t *testing.T) {
	t.Setenv(EnvAgentProfile, "")

	tests := map[string]string{
		"claude":           "claude",
		"openclaw":         "openclaw-legacy",
		"telegramcustomer": "telegramcustomer-legacy",
		"test":             "test",
	}

	for runtimeType, want := range tests {
		t.Run(runtimeType, func(t *testing.T) {
			t.Setenv("RUNTIME_TYPE", runtimeType)
			got := ResolveSelector(os.Getenv)
			if got != want {
				t.Fatalf("selector = %q, want %q", got, want)
			}
		})
	}
}

func TestResolveSelectorDefaultsToOpenclawLegacy(t *testing.T) {
	t.Setenv(EnvAgentProfile, "")
	t.Setenv(EnvRuntimeType, "")

	got := ResolveSelector(os.Getenv)
	if got != "openclaw-legacy" {
		t.Fatalf("selector = %q, want openclaw-legacy", got)
	}
}

func TestLoadProfileFromDir(t *testing.T) {
	root := t.TempDir()
	profileDir := filepath.Join(root, "claude")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatalf("mkdir profile dir failed: %v", err)
	}
	content := []byte(`name = "claude"
backend = "claude"
asset_root = "${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent"
role = "roles/claude.md"

[paths]
workspace = "${VMDOCKER_AGENT_WORKSPACE}"
home = "${VMDOCKER_RUNTIME_HOME}"
context = "${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/context"
memory = "${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/memory"
skills = "${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/skills"

[skills]
include = ["hymx-runtime"]
`)
	if err := os.WriteFile(filepath.Join(profileDir, "profile.toml"), content, 0o644); err != nil {
		t.Fatalf("write profile failed: %v", err)
	}

	got, err := Load(root, "claude")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if got.Name != "claude" {
		t.Fatalf("name = %q, want claude", got.Name)
	}
	if got.Backend != "claude" {
		t.Fatalf("backend = %q, want claude", got.Backend)
	}
	if len(got.Skills.Include) != 1 || got.Skills.Include[0] != "hymx-runtime" {
		t.Fatalf("skills = %#v, want hymx-runtime", got.Skills.Include)
	}
}

func TestLoadRejectsInvalidProfile(t *testing.T) {
	root := t.TempDir()
	profileDir := filepath.Join(root, "bad")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatalf("mkdir profile dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "profile.toml"), []byte(`name = "bad"`), 0o644); err != nil {
		t.Fatalf("write profile failed: %v", err)
	}

	if _, err := Load(root, "bad"); err == nil {
		t.Fatalf("expected invalid profile error")
	}
}

func TestLoadRejectsUnsafeProfileName(t *testing.T) {
	root := t.TempDir()
	escapeDir := filepath.Join(root, "..", "escape")
	if err := os.MkdirAll(escapeDir, 0o755); err != nil {
		t.Fatalf("mkdir escape dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(escapeDir, "profile.toml"), validProfile("escape"), 0o644); err != nil {
		t.Fatalf("write escape profile failed: %v", err)
	}

	tests := []string{
		"../escape",
		filepath.Join(string(filepath.Separator), "absolute"),
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Load(root, name)
			if err == nil {
				t.Fatalf("expected unsafe profile name error")
			}
			if errors.Is(err, os.ErrNotExist) {
				t.Fatalf("Load returned filesystem error %v, want unsafe name rejection", err)
			}
		})
	}
}

func validProfile(name string) []byte {
	return []byte(`name = "` + name + `"
backend = "` + name + `"
asset_root = "${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent"

[paths]
workspace = "${VMDOCKER_AGENT_WORKSPACE}"
home = "${VMDOCKER_RUNTIME_HOME}"
context = "${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/context"
memory = "${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/memory"
skills = "${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/skills"
`)
}
