package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultOpenclawConfigTemplatePath = "/app/openclaw.default.json"
	openclawStateDirName              = ".openclaw"
)

type EnvLookup func(string) string

type UserHomeDirFunc func() (string, error)

type OpenclawPaths struct {
	StateDir       string
	ConfigPath     string
	ConfigTemplate string
	LogDir         string
	GatewayLogPath string
	AgentWorkspace string
	HomeDir        string
	TmpDir         string
	XDGConfigHome  string
	XDGCacheHome   string
	XDGStateHome   string
}

func ResolveOpenclawStateDir(lookup EnvLookup, userHomeDir UserHomeDirFunc) string {
	if lookup == nil {
		lookup = os.Getenv
	}
	if userHomeDir == nil {
		userHomeDir = os.UserHomeDir
	}

	if stateDir := strings.TrimSpace(lookup("OPENCLAW_STATE_DIR")); stateDir != "" {
		return stateDir
	}
	if home := strings.TrimSpace(lookup("OPENCLAW_HOME")); home != "" && home != "/" {
		return filepath.Join(home, openclawStateDirName)
	}
	if home := strings.TrimSpace(lookup("HOME")); home != "" && home != "/" {
		return filepath.Join(home, openclawStateDirName)
	}
	if home, err := userHomeDir(); err == nil {
		home = strings.TrimSpace(home)
		if home != "" && home != "/" {
			return filepath.Join(home, openclawStateDirName)
		}
	}
	return filepath.Join("/tmp", openclawStateDirName)
}

func ResolveOpenclawConfigTemplatePath(lookup EnvLookup) string {
	if lookup == nil {
		lookup = os.Getenv
	}
	if path := strings.TrimSpace(lookup("OPENCLAW_CONFIG_TEMPLATE_PATH")); path != "" {
		return path
	}
	return DefaultOpenclawConfigTemplatePath
}

func ResolveOpenclawConfigPath(lookup EnvLookup, userHomeDir UserHomeDirFunc) string {
	if lookup == nil {
		lookup = os.Getenv
	}
	if path := strings.TrimSpace(lookup("OPENCLAW_CONFIG_PATH")); path != "" {
		return path
	}
	return filepath.Join(ResolveOpenclawStateDir(lookup, userHomeDir), "openclaw.json")
}

func ResolveOpenclawPaths(lookup EnvLookup, userHomeDir UserHomeDirFunc) OpenclawPaths {
	stateDir := ResolveOpenclawStateDir(lookup, userHomeDir)
	logDir := filepath.Join(stateDir, "logs")
	agentWorkspace := strings.TrimSpace(lookup("OPENCLAW_AGENT_WORKSPACE"))
	homeDir := strings.TrimSpace(lookup("HOME"))
	tmpDir := strings.TrimSpace(lookup("TMPDIR"))
	xdgConfigHome := strings.TrimSpace(lookup("XDG_CONFIG_HOME"))
	xdgCacheHome := strings.TrimSpace(lookup("XDG_CACHE_HOME"))
	xdgStateHome := strings.TrimSpace(lookup("XDG_STATE_HOME"))
	return OpenclawPaths{
		StateDir:       stateDir,
		ConfigPath:     ResolveOpenclawConfigPath(lookup, userHomeDir),
		ConfigTemplate: ResolveOpenclawConfigTemplatePath(lookup),
		LogDir:         logDir,
		GatewayLogPath: filepath.Join(logDir, "openclaw-gateway.log"),
		AgentWorkspace: agentWorkspace,
		HomeDir:        homeDir,
		TmpDir:         tmpDir,
		XDGConfigHome:  xdgConfigHome,
		XDGCacheHome:   xdgCacheHome,
		XDGStateHome:   xdgStateHome,
	}
}

func ensureDirs(dirs []string) error {
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir %s failed: %w", dir, err)
		}
		_ = os.Chmod(dir, 0o700)
	}
	return nil
}

func atomicWriteFile(targetPath string, data []byte, perm os.FileMode) error {
	tmpPath := targetPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, perm); err != nil {
		return fmt.Errorf("write tmp file failed: %w", err)
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("move tmp file failed: %w", err)
	}
	return nil
}

func EnsureOpenclawStateLayout(stateDir string) error {
	stateDir = strings.TrimSpace(stateDir)
	if stateDir == "" {
		return errors.New("state dir is empty")
	}

	if err := ensureDirs([]string{
		stateDir,
		filepath.Join(stateDir, "logs"),
		filepath.Join(stateDir, "agents"),
		filepath.Join(stateDir, "agents", "main"),
		filepath.Join(stateDir, "agents", "main", "sessions"),
		filepath.Join(stateDir, "agents", "main", "agent"),
	}); err != nil {
		return err
	}

	sessionsPath := filepath.Join(stateDir, "agents", "main", "sessions", "sessions.json")
	if _, err := os.Stat(sessionsPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat sessions path failed: %w", err)
	}

	if err := os.WriteFile(sessionsPath, []byte("{}\n"), 0o600); err != nil {
		return fmt.Errorf("write sessions store failed: %w", err)
	}
	return nil
}

func EnsureRuntimeDirectories(paths OpenclawPaths) error {
	return ensureDirs([]string{
		paths.HomeDir,
		paths.TmpDir,
		paths.XDGConfigHome,
		paths.XDGCacheHome,
		paths.XDGStateHome,
		paths.AgentWorkspace,
	})
}

func MaterializeOpenclawConfig(templatePath, targetPath, gatewayMode string) error {
	templatePath = strings.TrimSpace(templatePath)
	targetPath = strings.TrimSpace(targetPath)
	if templatePath == "" {
		return errors.New("config template path is empty")
	}
	if targetPath == "" {
		return errors.New("config target path is empty")
	}

	if _, err := os.Stat(targetPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat config path failed: %w", err)
	}

	raw, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("read config template failed: %w", err)
	}

	if strings.TrimSpace(gatewayMode) != "" {
		raw, err = overrideGatewayMode(raw, gatewayMode)
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create config dir failed: %w", err)
	}

	return atomicWriteFile(targetPath, raw, 0o600)
}

func ApplyManagedOpenclawConfig(targetPath string, paths OpenclawPaths) error {
	if strings.TrimSpace(paths.AgentWorkspace) == "" {
		return nil
	}

	raw, err := os.ReadFile(targetPath)
	if err != nil {
		return fmt.Errorf("read config for managed defaults failed: %w", err)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("decode config for managed defaults failed: %w", err)
	}

	defaults := ensureMap(ensureMap(cfg, "agents"), "defaults")
	defaults["workspace"] = paths.AgentWorkspace
	tools := ensureMap(cfg, "tools")
	fs := ensureMap(tools, "fs")
	fs["workspaceOnly"] = true

	formatted, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode managed defaults failed: %w", err)
	}
	formatted = append(formatted, '\n')
	if string(formatted) == string(raw) {
		return nil
	}

	return atomicWriteFile(targetPath, formatted, 0o600)
}

func PrepareOpenclawRuntime(lookup EnvLookup, userHomeDir UserHomeDirFunc) (OpenclawPaths, error) {
	if lookup == nil {
		lookup = os.Getenv
	}
	paths := ResolveOpenclawPaths(lookup, userHomeDir)
	if err := EnsureRuntimeDirectories(paths); err != nil {
		return OpenclawPaths{}, err
	}
	if err := EnsureOpenclawStateLayout(paths.StateDir); err != nil {
		return OpenclawPaths{}, err
	}
	if err := MaterializeOpenclawConfig(paths.ConfigTemplate, paths.ConfigPath, strings.TrimSpace(lookup("OPENCLAW_GATEWAY_MODE"))); err != nil {
		return OpenclawPaths{}, err
	}
	if err := ApplyManagedOpenclawConfig(paths.ConfigPath, paths); err != nil {
		return OpenclawPaths{}, err
	}
	return paths, nil
}

func ensureMap(root map[string]interface{}, key string) map[string]interface{} {
	if existing, ok := root[key].(map[string]interface{}); ok && existing != nil {
		return existing
	}
	next := map[string]interface{}{}
	root[key] = next
	return next
}

func overrideGatewayMode(raw []byte, gatewayMode string) ([]byte, error) {
	var cfg map[string]interface{}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("decode config template failed: %w", err)
	}

	gateway, ok := cfg["gateway"].(map[string]interface{})
	if !ok || gateway == nil {
		gateway = map[string]interface{}{}
		cfg["gateway"] = gateway
	}
	gateway["mode"] = gatewayMode

	formatted, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode config template failed: %w", err)
	}
	return append(formatted, '\n'), nil
}
