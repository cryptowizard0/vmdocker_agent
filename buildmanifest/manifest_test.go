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
runtime_profile = "claude"
dockerfile = "build/docker/Dockerfile.claude"
image_name = "chriswebber/docker-claude"
start_command = "sh -lc 'asset_root=\"${VMDOCKER_AGENT_ASSET_ROOT:-$VMDOCKER_RUNTIME_WORKSPACE/.vmdocker-agent}\"; bundle_root=\"${VMDOCKER_AGENT_BUNDLE_ROOT:-/opt/vmdocker-agent-bundle}\"; if [ ! -x \"$asset_root/bin/start-vmdocker-agent.sh\" ] && [ -d \"$bundle_root\" ]; then mkdir -p \"$asset_root\"; cp -R \"$bundle_root/.\" \"$asset_root/\"; chmod +x \"$asset_root/bin/start-vmdocker-agent.sh\"; fi; exec \"$asset_root/bin/start-vmdocker-agent.sh\"'"

[assets]
profiles = ["claude"]
skills = ["hymx-runtime"]
roles = ["claude"]
bootstrap = ["claude.sh"]

[env]
VMDOCKER_AGENT_PROFILE = "claude"
RUNTIME_TYPE = "claude"

[module_tags]
Sandbox-Agent = "shell"
Start-Command = "sh -lc 'asset_root=\"${VMDOCKER_AGENT_ASSET_ROOT:-$VMDOCKER_RUNTIME_WORKSPACE/.vmdocker-agent}\"; bundle_root=\"${VMDOCKER_AGENT_BUNDLE_ROOT:-/opt/vmdocker-agent-bundle}\"; if [ ! -x \"$asset_root/bin/start-vmdocker-agent.sh\" ] && [ -d \"$bundle_root\" ]; then mkdir -p \"$asset_root\"; cp -R \"$bundle_root/.\" \"$asset_root/\"; chmod +x \"$asset_root/bin/start-vmdocker-agent.sh\"; fi; exec \"$asset_root/bin/start-vmdocker-agent.sh\"'"
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
	if got.ModuleTags["Start-Command"] == "" {
		t.Fatalf("missing Start-Command module tag")
	}
	if got.Context != "." {
		t.Fatalf("context = %q, want default .", got.Context)
	}
}

func TestLoadBuildProfileWithBuildContexts(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "hermes.toml")
	if err := os.WriteFile(path, []byte(`name = "hermes"
runtime_profile = "hermes"
dockerfile = "Dockerfile.telegramcustomer"
context = "."
image_name = "sandytest456/docker-telegramcustomer:latest"
start_command = "sh -lc 'exec \"$VMDOCKER_RUNTIME_WORKSPACE/.vmdocker-agent/bin/start-vmdocker-agent.sh\"'"

[build_contexts]
extra_src = "${EXTRA_CONTEXT_PATH}"

[module_tags]
Start-Command = "sh -lc 'exec \"$VMDOCKER_RUNTIME_WORKSPACE/.vmdocker-agent/bin/start-vmdocker-agent.sh\"'"
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
runtime_profile = "bad"
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
