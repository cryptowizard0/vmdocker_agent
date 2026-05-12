package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cryptowizard0/vmdocker_agent/runtime/profile"
)

const (
	envRuntimeWorkspace = "VMDOCKER_RUNTIME_WORKSPACE"
	envAgentWorkspace   = "VMDOCKER_AGENT_WORKSPACE"
	envRuntimeHome      = "VMDOCKER_RUNTIME_HOME"

	assetRootName = ".vmdocker-agent"
)

type EnvLookup func(string) string

type Context struct {
	ProfileName string
	Backend     string
	AssetRoot   string
	Workspace   string
	Home        string
	ContextDir  string
	MemoryDir   string
	SkillsDir   string
	RolePath    string
	SkillPaths  []string
	Env         map[string]string
}

func Init(prof profile.Profile, lookup EnvLookup) (Context, error) {
	if lookup == nil {
		lookup = os.Getenv
	}

	runtimeRoot, err := runtimeWorkspace(lookup)
	if err != nil {
		return Context{}, err
	}
	assetRoot := normalizeAssetRoot(expand(prof.AssetRoot, lookup), runtimeRoot)
	workspace := expand(prof.Paths.Workspace, lookup)
	if workspace == "" {
		workspace = filepath.Join(runtimeRoot, "workspace")
	}
	home := expand(prof.Paths.Home, lookup)
	if home == "" {
		home = filepath.Join(runtimeRoot, ".home")
	}
	contextDir := normalizeAssetChild(expand(prof.Paths.Context, lookup), runtimeRoot, "context")
	memoryDir := normalizeAssetChild(expand(prof.Paths.Memory, lookup), runtimeRoot, "memory")
	skillsDir := normalizeAssetChild(expand(prof.Paths.Skills, lookup), runtimeRoot, "skills")

	for _, dir := range []string{assetRoot, workspace, home, contextDir, memoryDir, skillsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Context{}, fmt.Errorf("create harness dir %s failed: %w", dir, err)
		}
	}

	rolePath, err := resolveRolePath(assetRoot, expand(prof.Role, lookup))
	if err != nil {
		return Context{}, err
	}
	env := make(map[string]string, len(prof.Env)+7)
	for key, value := range prof.Env {
		env[key] = expand(value, lookup)
	}
	env[profile.EnvAgentProfile] = prof.Name
	env["VMDOCKER_AGENT_ASSET_ROOT"] = assetRoot
	env[profile.EnvProfileDir] = filepath.Join(assetRoot, "profiles", prof.Name)
	env["VMDOCKER_AGENT_SKILLS_DIR"] = skillsDir
	env["VMDOCKER_AGENT_ROLE_PATH"] = rolePath
	env["VMDOCKER_AGENT_CONTEXT_DIR"] = contextDir
	env["VMDOCKER_AGENT_MEMORY_DIR"] = memoryDir

	ctx := Context{
		ProfileName: prof.Name,
		Backend:     prof.Backend,
		AssetRoot:   assetRoot,
		Workspace:   workspace,
		Home:        home,
		ContextDir:  contextDir,
		MemoryDir:   memoryDir,
		SkillsDir:   skillsDir,
		RolePath:    rolePath,
		Env:         env,
	}

	for _, skill := range prof.Skills.Include {
		skill = strings.TrimSpace(skill)
		if skill == "" {
			continue
		}
		if err := validateSkillName(skill); err != nil {
			return Context{}, err
		}
		skillDir := filepath.Join(skillsDir, skill)
		if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
			if os.IsNotExist(err) {
				return Context{}, fmt.Errorf("missing skill %q at %s", skill, skillDir)
			}
			return Context{}, fmt.Errorf("check skill %q failed: %w", skill, err)
		}
		ctx.SkillPaths = append(ctx.SkillPaths, skillDir)
	}

	return ctx, nil
}

func runtimeWorkspace(lookup EnvLookup) (string, error) {
	if runtimeRoot := cleanRuntimeRoot(lookup(envRuntimeWorkspace)); runtimeRoot != "" {
		return runtimeRoot, nil
	}
	agentWorkspace := strings.TrimSpace(lookup(envAgentWorkspace))
	if agentWorkspace != "" {
		if runtimeRoot := cleanRuntimeRoot(filepath.Dir(agentWorkspace)); runtimeRoot != "" {
			return runtimeRoot, nil
		}
	}
	return "", fmt.Errorf("runtime workspace is required: set %s or a non-root %s parent", envRuntimeWorkspace, envAgentWorkspace)
}

func normalizeAssetRoot(path, runtimeRoot string) string {
	path = filepath.Clean(path)
	if path == "." || path == string(filepath.Separator)+assetRootName {
		return filepath.Join(runtimeRoot, assetRootName)
	}
	return path
}

func normalizeAssetChild(path, runtimeRoot, name string) string {
	path = filepath.Clean(path)
	if path == "." || path == filepath.Join(string(filepath.Separator), assetRootName, name) {
		return filepath.Join(runtimeRoot, assetRootName, name)
	}
	return path
}

func resolveRolePath(assetRoot, role string) (string, error) {
	role = strings.TrimSpace(role)
	if role == "" {
		return "", nil
	}
	if filepath.IsAbs(role) {
		return "", fmt.Errorf("invalid role path %q: absolute paths are not allowed", role)
	}
	cleanRole := filepath.Clean(role)
	if cleanRole == "." || cleanRole == ".." || strings.HasPrefix(cleanRole, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid role path %q: traversal is not allowed", role)
	}
	rolePath := filepath.Join(assetRoot, cleanRole)
	if !pathWithin(assetRoot, rolePath) {
		return "", fmt.Errorf("invalid role path %q: resolved path escapes asset root", role)
	}
	return rolePath, nil
}

func expand(value string, lookup EnvLookup) string {
	return os.Expand(value, func(name string) string {
		return lookup(name)
	})
}

func cleanRuntimeRoot(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	cleanPath := filepath.Clean(path)
	if cleanPath == string(filepath.Separator) || !filepath.IsAbs(cleanPath) {
		return ""
	}
	return cleanPath
}

func validateSkillName(skill string) error {
	if filepath.IsAbs(skill) || skill == "." || skill == ".." ||
		strings.ContainsRune(skill, filepath.Separator) ||
		strings.Contains(skill, "/") ||
		strings.Contains(skill, `\`) {
		return fmt.Errorf("invalid skill %q: skill names must not contain path traversal or separators", skill)
	}
	return nil
}

func pathWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
