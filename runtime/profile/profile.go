package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	EnvAgentProfile = "VMDOCKER_AGENT_PROFILE"
	EnvProfileDir   = "VMDOCKER_AGENT_PROFILE_DIR"
	EnvRuntimeType  = "RUNTIME_TYPE"

	DefaultProfileDir = "harness/profiles"

	ProfileClaude                 = "claude"
	ProfileOpenclawLegacy         = "openclaw-legacy"
	ProfileTelegramCustomerLegacy = "telegramcustomer-legacy"
	ProfileTest                   = "test"
)

type EnvLookup func(string) string

type Profile struct {
	Name      string            `toml:"name"`
	Backend   string            `toml:"backend"`
	AssetRoot string            `toml:"asset_root"`
	Role      string            `toml:"role"`
	Paths     Paths             `toml:"paths"`
	Skills    Skills            `toml:"skills"`
	Env       map[string]string `toml:"env"`
}

type Paths struct {
	Workspace string `toml:"workspace"`
	Home      string `toml:"home"`
	Context   string `toml:"context"`
	Memory    string `toml:"memory"`
	Skills    string `toml:"skills"`
}

type Skills struct {
	Include []string `toml:"include"`
}

func ResolveSelector(lookup EnvLookup) string {
	if lookup == nil {
		lookup = os.Getenv
	}
	if selected := strings.TrimSpace(lookup(EnvAgentProfile)); selected != "" {
		return selected
	}

	switch runtimeType := strings.TrimSpace(lookup(EnvRuntimeType)); runtimeType {
	case ProfileClaude:
		return ProfileClaude
	case "telegramcustomer":
		return ProfileTelegramCustomerLegacy
	case ProfileTest:
		return ProfileTest
	case "openclaw", "":
		return ProfileOpenclawLegacy
	default:
		return runtimeType
	}
}

func ResolveProfileDir(lookup EnvLookup) string {
	if lookup == nil {
		lookup = os.Getenv
	}
	if dir := strings.TrimSpace(lookup(EnvProfileDir)); dir != "" {
		return dir
	}
	return DefaultProfileDir
}

func Load(root, name string) (Profile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Profile{}, fmt.Errorf("profile name is empty")
	}
	path := filepath.Join(root, name, "profile.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, fmt.Errorf("read profile %s failed: %w", path, err)
	}

	var prof Profile
	if err := toml.Unmarshal(data, &prof); err != nil {
		return Profile{}, fmt.Errorf("parse profile %s failed: %w", path, err)
	}
	if err := prof.Validate(); err != nil {
		return Profile{}, fmt.Errorf("invalid profile %s: %w", path, err)
	}
	return prof, nil
}

func (p Profile) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(p.Backend) == "" {
		return fmt.Errorf("backend is required")
	}
	if strings.TrimSpace(p.AssetRoot) == "" {
		return fmt.Errorf("asset_root is required")
	}
	if strings.TrimSpace(p.Paths.Workspace) == "" {
		return fmt.Errorf("paths.workspace is required")
	}
	if strings.TrimSpace(p.Paths.Home) == "" {
		return fmt.Errorf("paths.home is required")
	}
	if strings.TrimSpace(p.Paths.Context) == "" {
		return fmt.Errorf("paths.context is required")
	}
	if strings.TrimSpace(p.Paths.Memory) == "" {
		return fmt.Errorf("paths.memory is required")
	}
	if strings.TrimSpace(p.Paths.Skills) == "" {
		return fmt.Errorf("paths.skills is required")
	}
	return nil
}
