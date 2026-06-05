package buildmanifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const DefaultStartCommand = "/usr/local/bin/start-vmdocker-agent-workspace.sh"

type Manifest struct {
	Name           string            `toml:"name"`
	RuntimeProfile string            `toml:"runtime_profile"`
	Dockerfile     string            `toml:"dockerfile"`
	Context        string            `toml:"context"`
	ImageName      string            `toml:"image_name"`
	StartCommand   string            `toml:"start_command"`
	Assets         Assets            `toml:"assets"`
	Env            map[string]string `toml:"env"`
	BuildContexts  map[string]string `toml:"build_contexts"`
	ModuleTags     map[string]string `toml:"module_tags"`
}

type Assets struct {
	Bootstrap []string `toml:"bootstrap"`
}

func Load(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read build manifest %s failed: %w", path, err)
	}
	var manifest Manifest
	if err := toml.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse build manifest %s failed: %w", path, err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, fmt.Errorf("invalid build manifest %s: %w", path, err)
	}
	return manifest, nil
}

func (m *Manifest) Validate() error {
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(m.RuntimeProfile) == "" {
		return fmt.Errorf("runtime_profile is required")
	}
	if _, err := m.RuntimeProfileName(); err != nil {
		return err
	}
	if strings.TrimSpace(m.Dockerfile) == "" {
		return fmt.Errorf("dockerfile is required")
	}
	if strings.TrimSpace(m.Context) == "" {
		m.Context = "."
	}
	if strings.TrimSpace(m.ImageName) == "" {
		return fmt.Errorf("image_name is required")
	}
	if strings.TrimSpace(m.StartCommand) == "" {
		m.StartCommand = DefaultStartCommand
	}
	if m.StartCommand != DefaultStartCommand && !strings.Contains(m.StartCommand, "VMDOCKER_RUNTIME_WORKSPACE") {
		return fmt.Errorf("start_command must resolve through VMDOCKER_RUNTIME_WORKSPACE")
	}
	if tag := m.ModuleTags["Start-Command"]; strings.TrimSpace(tag) != "" {
		return fmt.Errorf("module Start-Command is derived from start_command")
	}
	for key := range m.Env {
		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("env key is required")
		}
		if key == "VMDOCKER_AGENT_PROFILE" {
			return fmt.Errorf("env VMDOCKER_AGENT_PROFILE is derived from runtime_profile")
		}
	}
	return nil
}

func (m Manifest) RuntimeProfileName() (string, error) {
	profilePath := strings.TrimSpace(m.RuntimeProfile)
	if profilePath == "" {
		return "", fmt.Errorf("runtime_profile is required")
	}
	if filepath.IsAbs(profilePath) || strings.Contains(profilePath, `\`) {
		return "", fmt.Errorf("runtime_profile must be a relative profile.toml path")
	}
	cleanPath := filepath.Clean(profilePath)
	if cleanPath == "." || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("runtime_profile must be a relative profile.toml path")
	}
	if filepath.Base(cleanPath) != "profile.toml" || filepath.Dir(cleanPath) == "." {
		return "", fmt.Errorf("runtime_profile must be a relative profile.toml path")
	}
	name := filepath.Base(filepath.Dir(cleanPath))
	if name == "." || name == ".." || strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("runtime_profile must be a relative profile.toml path")
	}
	return name, nil
}
