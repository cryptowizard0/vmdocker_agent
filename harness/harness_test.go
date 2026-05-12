package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cryptowizard0/vmdocker_agent/runtime/profile"
)

func TestInitCreatesWorkspaceScopedHarnessDirs(t *testing.T) {
	tempDir := t.TempDir()
	runtimeWorkspace := filepath.Join(tempDir, "runtime-workspace")
	agentWorkspace := filepath.Join(runtimeWorkspace, "workspace")
	runtimeHome := filepath.Join(runtimeWorkspace, ".home")
	skillsDir := filepath.Join(runtimeWorkspace, ".vmdocker-agent", "skills")
	rolePath := filepath.Join(runtimeWorkspace, ".vmdocker-agent", "roles", "claude.md")

	if err := os.MkdirAll(filepath.Join(skillsDir, "hymx-runtime"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "hymx-runtime", "SKILL.md"), []byte("---\nname: hymx-runtime\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(rolePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rolePath, []byte("# Claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	prof := profile.Profile{
		Name:      "claude",
		Backend:   "claude",
		AssetRoot: "${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent",
		Role:      "roles/claude.md",
		Paths: profile.Paths{
			Workspace: "${VMDOCKER_AGENT_WORKSPACE}",
			Home:      "${VMDOCKER_RUNTIME_HOME}",
			Context:   "${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/context",
			Memory:    "${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/memory",
			Skills:    "${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/skills",
		},
		Skills: profile.Skills{Include: []string{"hymx-runtime"}},
		Env: map[string]string{
			"CUSTOM_HOME": "${VMDOCKER_RUNTIME_HOME}",
		},
	}
	lookup := mapLookup(map[string]string{
		"VMDOCKER_RUNTIME_WORKSPACE": runtimeWorkspace,
		"VMDOCKER_AGENT_WORKSPACE":   agentWorkspace,
		"VMDOCKER_RUNTIME_HOME":      runtimeHome,
	})

	ctx, err := Init(prof, lookup)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	wantAssetRoot := filepath.Join(runtimeWorkspace, ".vmdocker-agent")
	if ctx.AssetRoot != wantAssetRoot {
		t.Fatalf("AssetRoot = %q, want %q", ctx.AssetRoot, wantAssetRoot)
	}
	if ctx.Workspace != agentWorkspace {
		t.Fatalf("Workspace = %q, want %q", ctx.Workspace, agentWorkspace)
	}
	if ctx.Home != runtimeHome {
		t.Fatalf("Home = %q, want %q", ctx.Home, runtimeHome)
	}
	for name, dir := range map[string]string{
		"AssetRoot":  ctx.AssetRoot,
		"Workspace":  ctx.Workspace,
		"Home":       ctx.Home,
		"ContextDir": ctx.ContextDir,
		"MemoryDir":  ctx.MemoryDir,
		"SkillsDir":  ctx.SkillsDir,
	} {
		if !strings.HasPrefix(dir, runtimeWorkspace+string(os.PathSeparator)) && dir != runtimeWorkspace {
			t.Fatalf("%s = %q, want under %q", name, dir, runtimeWorkspace)
		}
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("%s was not created: %v", name, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s = %q, want directory", name, dir)
		}
	}
	if ctx.RolePath != rolePath {
		t.Fatalf("RolePath = %q, want %q", ctx.RolePath, rolePath)
	}
	wantSkill := filepath.Join(skillsDir, "hymx-runtime")
	if len(ctx.SkillPaths) != 1 || ctx.SkillPaths[0] != wantSkill {
		t.Fatalf("SkillPaths = %#v, want [%q]", ctx.SkillPaths, wantSkill)
	}
}

func TestInitRejectsMissingSkill(t *testing.T) {
	runtimeWorkspace := t.TempDir()
	prof := profile.Profile{
		Name:      "claude",
		Backend:   "claude",
		AssetRoot: "${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent",
		Role:      "roles/claude.md",
		Paths: profile.Paths{
			Workspace: "${VMDOCKER_RUNTIME_WORKSPACE}/workspace",
			Home:      "${VMDOCKER_RUNTIME_WORKSPACE}/.home",
			Context:   "${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/context",
			Memory:    "${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/memory",
			Skills:    "${VMDOCKER_RUNTIME_WORKSPACE}/.vmdocker-agent/skills",
		},
		Skills: profile.Skills{Include: []string{"missing-skill"}},
	}

	_, err := Init(prof, mapLookup(map[string]string{
		"VMDOCKER_RUNTIME_WORKSPACE": runtimeWorkspace,
	}))
	if err == nil {
		t.Fatal("Init() error = nil, want missing skill error")
	}
	if !strings.Contains(err.Error(), "missing skill") {
		t.Fatalf("Init() error = %q, want missing skill", err)
	}
}

func mapLookup(values map[string]string) EnvLookup {
	return func(name string) string {
		return values[name]
	}
}
