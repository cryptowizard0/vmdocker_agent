package buildmanifest

import (
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type Manifest struct {
	Name           string            `toml:"name"`
	RuntimeProfile string            `toml:"runtime_profile"`
	Dockerfile     string            `toml:"dockerfile"`
	ImageName      string            `toml:"image_name"`
	StartCommand   string            `toml:"start_command"`
	Assets         Assets            `toml:"assets"`
	Env            map[string]string `toml:"env"`
	ModuleTags     map[string]string `toml:"module_tags"`
}

type Assets struct {
	Profiles  []string `toml:"profiles"`
	Skills    []string `toml:"skills"`
	Roles     []string `toml:"roles"`
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

func (m Manifest) Validate() error {
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(m.RuntimeProfile) == "" {
		return fmt.Errorf("runtime_profile is required")
	}
	if strings.TrimSpace(m.Dockerfile) == "" {
		return fmt.Errorf("dockerfile is required")
	}
	if strings.TrimSpace(m.ImageName) == "" {
		return fmt.Errorf("image_name is required")
	}
	if !strings.Contains(m.StartCommand, "VMDOCKER_RUNTIME_WORKSPACE") {
		return fmt.Errorf("start_command must resolve through VMDOCKER_RUNTIME_WORKSPACE")
	}
	if tag := m.ModuleTags["Start-Command"]; tag != "" && !strings.Contains(tag, "VMDOCKER_RUNTIME_WORKSPACE") {
		return fmt.Errorf("module Start-Command must resolve through VMDOCKER_RUNTIME_WORKSPACE")
	}
	return nil
}
