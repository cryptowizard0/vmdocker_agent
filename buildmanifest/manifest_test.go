package buildmanifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadBuildProfile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "claude.toml")
	if err := os.WriteFile(path, []byte(`name = "claude"
runtime_profile = "harness/profiles/claude/profile.toml"
dockerfile = "build/docker/Dockerfile.claude"
image_name = "chriswebber/docker-claude"

[assets]
bootstrap = ["claude.sh"]

[env]
RUNTIME_TYPE = "claude"
`), 0o644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if got.Name != "claude" {
		t.Fatalf("name = %q", got.Name)
	}
	if got.ModuleTags["Start-Command"] != "" {
		t.Fatalf("unexpected Start-Command module tag")
	}
	if got.Context != "." {
		t.Fatalf("context = %q, want default .", got.Context)
	}
	if got.StartCommand != DefaultStartCommand {
		t.Fatalf("start_command = %q, want default %q", got.StartCommand, DefaultStartCommand)
	}
	profileName, err := got.RuntimeProfileName()
	if err != nil {
		t.Fatalf("RuntimeProfileName failed: %v", err)
	}
	if profileName != "claude" {
		t.Fatalf("runtime profile name = %q", profileName)
	}
}

func TestLoadBuildProfileWithBuildContexts(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "hermes.toml")
	if err := os.WriteFile(path, []byte(`name = "hermes"
runtime_profile = "harness/profiles/hermes/profile.toml"
dockerfile = "Dockerfile.telegramcustomer"
image_name = "sandytest456/docker-telegramcustomer:latest"

[build_contexts]
extra_src = "${EXTRA_CONTEXT_PATH}"
`), 0o644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if got.BuildContexts["extra_src"] != "${EXTRA_CONTEXT_PATH}" {
		t.Fatalf("extra_src build context = %q", got.BuildContexts["extra_src"])
	}
}

func TestLoadRejectsSystemPathStartCommand(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bad.toml")
	if err := os.WriteFile(path, []byte(`name = "bad"
runtime_profile = "harness/profiles/bad/profile.toml"
dockerfile = "Dockerfile"
image_name = "example/bad"
start_command = "/usr/local/bin/start-vmdocker-agent.sh"
`), 0o644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "VMDOCKER_RUNTIME_WORKSPACE") {
		t.Fatalf("expected workspace start command error, got %v", err)
	}
}

func TestLoadRejectsPlainRuntimeProfileName(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bad.toml")
	if err := os.WriteFile(path, []byte(`name = "bad"
runtime_profile = "bad"
dockerfile = "Dockerfile"
image_name = "example/bad"
`), 0o644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "runtime_profile must be a relative profile.toml path") {
		t.Fatalf("expected runtime_profile path error, got %v", err)
	}
}

func TestLoadRejectsModuleStartCommand(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bad.toml")
	if err := os.WriteFile(path, []byte(`name = "bad"
runtime_profile = "harness/profiles/bad/profile.toml"
dockerfile = "Dockerfile"
image_name = "example/bad"

[module_tags]
Start-Command = "sh -lc 'exec \"$VMDOCKER_RUNTIME_WORKSPACE/.vmdocker-agent/bin/start-vmdocker-agent.sh\"'"
`), 0o644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "derived from start_command") {
		t.Fatalf("expected derived Start-Command error, got %v", err)
	}
}

func TestLoadRejectsDerivedAgentProfileEnv(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bad.toml")
	if err := os.WriteFile(path, []byte(`name = "bad"
runtime_profile = "harness/profiles/bad/profile.toml"
dockerfile = "Dockerfile"
image_name = "example/bad"

[env]
VMDOCKER_AGENT_PROFILE = "other"
`), 0o644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "runtime_profile") {
		t.Fatalf("expected derived profile env error, got %v", err)
	}
}
